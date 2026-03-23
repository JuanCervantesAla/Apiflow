package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"html"
	"net/smtp"
	"os"
	"strings"
	"time"

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
	params.Body = sanitizeAILeadIn(params.Body)
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
	msg := buildEmailMessage(params.From, toList, ccList, params.Subject, params.Body, params.FromName)

	// Authenticate
	auth := smtp.PlainAuth("", params.SMTPUser, params.SMTPPassword, params.SMTPHost)

	// Combine all recipients for SMTP
	allRecipients := append(toList, ccList...)
	allRecipients = append(allRecipients, bccList...)

	// Send email
	addr := fmt.Sprintf("%s:%d", params.SMTPHost, params.SMTPPort)
	err := smtp.SendMail(addr, auth, params.From, allRecipients, []byte(msg))

	output := map[string]interface{}{
		"sent":             err == nil,
		"provider":         "smtp",
		"recipients":       toList,
		"cc":               ccList,
		"bcc":              bccList,
		"subject":          params.Subject,
		"interpolatedBody": params.Body,
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
	message.AddContent(mail.NewContent("text/html", renderProfessionalEmailHTML(params.Subject, params.Body, fromName)))

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
		"sent":                 resp.StatusCode >= 200 && resp.StatusCode < 300,
		"provider":             "sendgrid-api",
		"recipients":           toList,
		"cc":                   ccList,
		"bcc":                  bccList,
		"subject":              params.Subject,
		"interpolatedBody":     params.Body,
		"statusCode":           resp.StatusCode,
		"response":             resp.Body,
		"providerResponseBody": resp.Body,
		"headers":              resp.Headers,
		"region":               region,
	}

	if resp.StatusCode == 202 && strings.TrimSpace(resp.Body) == "" {
		output["providerMessage"] = "Accepted by SendGrid (202). Empty body is expected for this provider."
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
func buildEmailMessage(from string, to []string, cc []string, subject string, body string, fromName string) string {
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(to, ", ")
	if len(cc) > 0 {
		headers["Cc"] = strings.Join(cc, ", ")
	}
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	boundary := "capyflow-boundary"
	headers["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=%s", boundary)

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}

	htmlBody := renderProfessionalEmailHTML(subject, body, fromName)
	message += "\r\n"
	message += fmt.Sprintf("--%s\r\n", boundary)
	message += "Content-Type: text/plain; charset=utf-8\r\n"
	message += "Content-Transfer-Encoding: 8bit\r\n\r\n"
	message += body + "\r\n"
	message += fmt.Sprintf("--%s\r\n", boundary)
	message += "Content-Type: text/html; charset=utf-8\r\n"
	message += "Content-Transfer-Encoding: 8bit\r\n\r\n"
	message += htmlBody + "\r\n"
	message += fmt.Sprintf("--%s--\r\n", boundary)

	return message
}

func renderProfessionalEmailHTML(subject, body, fromName string) string {
	safeSubject := html.EscapeString(strings.TrimSpace(subject))
	if safeSubject == "" {
		safeSubject = "Notification"
	}

	brand := html.EscapeString(strings.TrimSpace(fromName))
	if brand == "" {
		brand = "CapyFlow"
	}

	content := renderBodyContentHTML(sanitizeAILeadIn(body))
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:Arial,Helvetica,sans-serif;color:#111827;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="padding:24px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:640px;background:#ffffff;border:1px solid #e5e7eb;border-radius:10px;overflow:hidden;">
          <tr>
						<td style="padding:14px 20px;background:#f59e0b;color:#111827;border-bottom:2px solid #111827;">
							<div style="font-size:12px;letter-spacing:1px;text-transform:uppercase;font-weight:700;">CapyFlow Notification</div>
						</td>
					</tr>
					<tr>
						<td style="padding:16px 20px;background:#111827;color:#ffffff;">
							<div style="font-size:11px;letter-spacing:0.8px;text-transform:uppercase;opacity:0.85;">From %s</div>
              <div style="margin-top:6px;font-size:20px;line-height:1.3;font-weight:700;">%s</div>
            </td>
          </tr>
          <tr>
            <td style="padding:22px 20px;font-size:14px;line-height:1.6;color:#111827;">
              %s
            </td>
          </tr>
          <tr>
            <td style="padding:12px 20px;background:#f9fafb;border-top:1px solid #e5e7eb;font-size:12px;color:#6b7280;">
              Powered by CapyFlow · %d · %s
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, safeSubject, brand, safeSubject, content, time.Now().Year(), brand)
}

func sanitizeAILeadIn(text string) string {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return cleaned
	}

	prefixes := []string{
		"hello, this is an ai-generated summary:",
		"hello this is an ai-generated summary:",
		"this is an ai-generated summary:",
	}

	lower := strings.ToLower(cleaned)
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			cleaned = strings.TrimSpace(cleaned[len(prefix):])
			lower = strings.ToLower(cleaned)
		}
	}

	return cleaned
}

func renderBodyContentHTML(body string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var out strings.Builder
	var paragraph []string
	inList := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		out.WriteString(`<p style="margin:0 0 12px 0;">`)
		out.WriteString(strings.Join(paragraph, "<br/>"))
		out.WriteString(`</p>`)
		paragraph = paragraph[:0]
	}

	closeList := func() {
		if !inList {
			return
		}
		out.WriteString(`</ul>`)
		inList = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flushParagraph()
			closeList()
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
			flushParagraph()
			if !inList {
				out.WriteString(`<ul style="margin:0 0 12px 18px;padding:0;">`)
				inList = true
			}
			item := strings.TrimSpace(line[2:])
			out.WriteString(`<li style="margin:0 0 6px 0;">`)
			out.WriteString(html.EscapeString(item))
			out.WriteString(`</li>`)
			continue
		}

		closeList()
		paragraph = append(paragraph, html.EscapeString(line))
	}

	flushParagraph()
	closeList()

	if strings.TrimSpace(out.String()) == "" {
		return `<p style="margin:0;">No message content.</p>`
	}

	return out.String()
}
