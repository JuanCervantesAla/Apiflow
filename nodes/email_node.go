package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
)

type EmailNode struct{}

type EmailParams struct {
	To           interface{} `json:"to"`           // Can be string or array
	Subject      string      `json:"subject"`
	Body         string      `json:"body"`
	From         string      `json:"from"`
	SMTPHost     string      `json:"smtpHost"`
	SMTPPort     int         `json:"smtpPort"`
	SMTPUser     string      `json:"smtpUser"`
	SMTPPassword string      `json:"smtpPassword"`
	CC           interface{} `json:"cc"`  // Optional
	BCC          interface{} `json:"bcc"` // Optional
}

func (n *EmailNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params EmailParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error parsing email params: %v", err)
	}

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
