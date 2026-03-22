package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type EmailNode struct{}

type EmailParams struct {
	Provider         string      `json:"provider"` // "smtp" | "sendgrid-api"
	To               interface{} `json:"to"`       // Can be string or array
	Subject          string      `json:"subject"`
	Body             string      `json:"body"`
	From             string      `json:"from"`
	FromName         string      `json:"fromName"`
	SMTPHost         string      `json:"smtpHost"`
	SMTPPort         int         `json:"smtpPort"`
	SMTPUser         string      `json:"smtpUser"`
	SMTPPassword     string      `json:"smtpPassword"`
	SendGridAPIToken string      `json:"sendgridApiToken"`
	SendGridRegion   string      `json:"sendgridDataResidency"` // "" | "eu"
	CC               interface{} `json:"cc"`                    // Optional
	BCC              interface{} `json:"bcc"`                   // Optional
}

func (n *EmailNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params EmailParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error parsing email params: %v", err)
	}

	// Interpolate dynamic values from previous node outputs in subject and body
	params.Subject = interpolateString(params.Subject, prev)
	params.Body = interpolateString(params.Body, prev)
	params.From = interpolateString(params.From, prev)
	params.FromName = interpolateString(params.FromName, prev)
	params.Provider = strings.ToLower(strings.TrimSpace(interpolateString(params.Provider, prev)))
	params.SendGridAPIToken = strings.TrimSpace(interpolateString(params.SendGridAPIToken, prev))
	params.SendGridRegion = strings.ToLower(strings.TrimSpace(interpolateString(params.SendGridRegion, prev)))

	// Parse recipients (to, cc, bcc)
	toList := parseRecipients(params.To)
	ccList := parseRecipients(params.CC)
	bccList := parseRecipients(params.BCC)

	if len(toList) == 0 {
		return nil, fmt.Errorf("at least one recipient (to) is required")
	}

	if params.Subject == "" {
		return nil, fmt.Errorf("subject is required")
	}

	if params.Body == "" {
		return nil, fmt.Errorf("body is required")
	}

	// Default mode: prefer SendGrid API if an API key is present.
	if params.Provider == "" {
		if strings.TrimSpace(os.Getenv("SENDGRID_API_KEY")) != "" {
			params.Provider = "sendgrid-api"
		} else {
			params.Provider = "smtp"
		}
	}

	if params.Provider == "sendgrid-api" {
		if params.From == "" {
			params.From = strings.TrimSpace(os.Getenv("SENDGRID_SENDER_EMAIL"))
		}
		if params.FromName == "" {
			params.FromName = strings.TrimSpace(os.Getenv("SENDGRID_SENDER_NAME"))
		}
		if params.From == "" {
			return nil, fmt.Errorf("from address is required (node field 'from' or SENDGRID_SENDER_EMAIL env var)")
		}
		return sendWithSendGridAPI(params, toList, ccList, bccList)
	}

	if params.From == "" {
		return nil, fmt.Errorf("from address is required")
	}

	if params.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}

	if params.SMTPPort == 0 {
		params.SMTPPort = 587 // Default to submission port
	}

	// Build email message
	msg := buildEmailMessage(params.From, toList, ccList, params.Subject, params.Body)

	// Authenticate
	auth := smtp.PlainAuth("", params.SMTPUser, params.SMTPPassword, params.SMTPHost)

	// Combine all recipients for SMTP
	allRecipients := append(toList, ccList...)
	allRecipients = append(allRecipients, bccList...)

	// Send email
	addr := fmt.Sprintf("%s:%d", params.SMTPHost, params.SMTPPort)
	err := smtp.SendMail(addr, auth, params.From, allRecipients, []byte(msg))

	output := map[string]interface{}{
		"sent":       err == nil,
		"recipients": toList,
		"cc":         ccList,
		"bcc":        bccList,
		"subject":    params.Subject,
	}

	if err != nil {
		output["error"] = err.Error()
		return output, fmt.Errorf("failed to send email: %v", err)
	}

	return output, nil
}

func sendWithSendGridAPI(params EmailParams, toList, ccList, bccList []string) (map[string]interface{}, error) {
	token := strings.TrimSpace(params.SendGridAPIToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("SENDGRID_API_KEY"))
	}
	if token == "" {
		return nil, fmt.Errorf("SENDGRID_API_KEY is required (node parameter or environment variable)")
	}

	fromName := strings.TrimSpace(params.FromName)
	if fromName == "" {
		fromName = "CapyFlow"
	}

	from := mail.NewEmail(fromName, params.From)
	message := mail.NewV3Mail()
	message.SetFrom(from)
	message.Subject = params.Subject

	personalization := mail.NewPersonalization()
	for _, recipient := range toList {
		personalization.AddTos(mail.NewEmail("", recipient))
	}
	for _, cc := range ccList {
		personalization.AddCCs(mail.NewEmail("", cc))
	}
	for _, bcc := range bccList {
		personalization.AddBCCs(mail.NewEmail("", bcc))
	}
	message.AddPersonalizations(personalization)

	message.AddContent(mail.NewContent("text/plain", params.Body))
	message.AddContent(mail.NewContent("text/html", fmt.Sprintf("<p>%s</p>", params.Body)))

	client := sendgrid.NewSendClient(token)
	region := strings.ToLower(strings.TrimSpace(params.SendGridRegion))
	if region == "eu" {
		// EU data residency (for EU-pinned subusers).
		request, err := sendgrid.SetDataResidency(client.Request, "eu")
		if err != nil {
			return nil, fmt.Errorf("failed to set SendGrid data residency: %v", err)
		}
		client.Request = request
	}

	resp, err := client.Send(message)
	if err != nil {
		return nil, fmt.Errorf("failed to call SendGrid API: %v", err)
	}

	output := map[string]interface{}{
		"sent":       resp.StatusCode >= 200 && resp.StatusCode < 300,
		"provider":   "sendgrid-api",
		"recipients": toList,
		"cc":         ccList,
		"bcc":        bccList,
		"subject":    params.Subject,
		"statusCode": resp.StatusCode,
		"response":   resp.Body,
		"headers":    resp.Headers,
		"region":     region,
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return output, fmt.Errorf("sendgrid API error (status %d): %s", resp.StatusCode, resp.Body)
	}

	return output, nil
}

// parseRecipients converts string or array to []string
func parseRecipients(input interface{}) []string {
	if input == nil {
		return []string{}
	}

	switch v := input.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		// Split by comma for multiple recipients
		parts := strings.Split(v, ",")
		result := []string{}
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	case []interface{}:
		result := []string{}
		for _, item := range v {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, strings.TrimSpace(str))
			}
		}
		return result
	case []string:
		return v
	default:
		return []string{}
	}
}

// buildEmailMessage constructs RFC 5322 compliant email
func buildEmailMessage(from string, to []string, cc []string, subject string, body string) string {
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(to, ", ")
	if len(cc) > 0 {
		headers["Cc"] = strings.Join(cc, ", ")
	}
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/plain; charset=utf-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	return message
}
