package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FinanceExtractNode struct{}

type FinanceExtractParams struct {
	Text            string `json:"text"`
	DefaultCurrency string `json:"defaultCurrency"`
}

func (n *FinanceExtractNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params FinanceExtractParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error parsing finance-extract params: %v", err)
	}

	params.Text = interpolateString(params.Text, prev)
	params.Text = strings.TrimSpace(params.Text)
	if params.Text == "" {
		return nil, fmt.Errorf("text is required")
	}

	currency := strings.TrimSpace(strings.ToUpper(params.DefaultCurrency))
	if currency == "" {
		currency = "USD"
	}

	amount, amountFound := extractAmount(params.Text)
	merchant := extractMerchant(params.Text)
	dateISO, dateFound := extractDate(params.Text)
	detectedCurrency := extractCurrency(params.Text)
	if detectedCurrency == "" {
		detectedCurrency = currency
	}
	financeType := detectFinanceType(params.Text, amount)
	category := detectCategory(params.Text)

	confidence := 0.35
	if amountFound {
		confidence += 0.3
	}
	if merchant != "" {
		confidence += 0.15
	}
	if dateFound {
		confidence += 0.1
	}
	if category != "other" {
		confidence += 0.1
	}
	if confidence > 0.99 {
		confidence = 0.99
	}

	transaction := map[string]interface{}{
		"amount":      amount,
		"currency":    detectedCurrency,
		"merchant":    merchant,
		"date":        dateISO,
		"category":    category,
		"financeType": financeType,
		"confidence":  confidence,
		"rawText":     params.Text,
	}

	return map[string]interface{}{
		"transaction": transaction,
		"extracted":   true,
	}, nil
}

func extractAmount(text string) (float64, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:total|amount|paid|charge|monto|importe)\s*[:=]?\s*([$€£]?\s*[0-9]+(?:[.,][0-9]{2})?)`),
		regexp.MustCompile(`(?i)([$€£]\s*[0-9]+(?:[.,][0-9]{2})?)`),
		regexp.MustCompile(`(?i)\b([0-9]+(?:[.,][0-9]{2}))\b`),
	}

	for _, re := range patterns {
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			continue
		}
		candidate := strings.TrimSpace(m[1])
		candidate = strings.TrimLeft(candidate, "$€£")
		candidate = strings.ReplaceAll(candidate, ",", ".")
		v, err := strconv.ParseFloat(candidate, 64)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

func extractMerchant(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if clean == "" {
			continue
		}
		lower := strings.ToLower(clean)
		if strings.Contains(lower, "total") || strings.Contains(lower, "amount") || strings.Contains(lower, "tax") || strings.Contains(lower, "iva") {
			continue
		}
		if len(clean) > 2 {
			return clean
		}
	}
	return ""
}

func extractDate(text string) (string, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`),
		regexp.MustCompile(`\b(\d{2}/\d{2}/\d{4})\b`),
		regexp.MustCompile(`\b(\d{2}-\d{2}-\d{4})\b`),
	}
	layouts := []string{"2006-01-02", "02/01/2006", "02-01-2006"}

	for i, re := range patterns {
		m := re.FindStringSubmatch(text)
		if len(m) < 2 {
			continue
		}
		t, err := time.Parse(layouts[i], m[1])
		if err == nil {
			return t.Format(time.RFC3339), true
		}
	}
	return time.Now().Format(time.RFC3339), false
}

func extractCurrency(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "usd") || strings.Contains(text, "$"):
		return "USD"
	case strings.Contains(lower, "eur") || strings.Contains(text, "€"):
		return "EUR"
	case strings.Contains(lower, "gbp") || strings.Contains(text, "£"):
		return "GBP"
	case strings.Contains(lower, "clp"):
		return "CLP"
	case strings.Contains(lower, "mxn"):
		return "MXN"
	default:
		return ""
	}
}

func detectFinanceType(text string, amount float64) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "refund") || strings.Contains(lower, "salary") || strings.Contains(lower, "income") || strings.Contains(lower, "deposit") || amount < 0 {
		return "income"
	}
	return "expense"
}

func detectCategory(text string) string {
	lower := strings.ToLower(text)
	categories := map[string][]string{
		"food":          {"restaurant", "food", "cafe", "coffee", "grocery", "supermarket"},
		"transport":     {"uber", "taxi", "bus", "metro", "fuel", "gas"},
		"housing":       {"rent", "electricity", "water", "internet", "mortgage"},
		"health":        {"pharmacy", "clinic", "hospital", "doctor"},
		"entertainment": {"netflix", "spotify", "cinema", "movie", "game"},
		"shopping":      {"store", "shop", "amazon", "mall"},
	}

	for category, words := range categories {
		for _, word := range words {
			if strings.Contains(lower, word) {
				return category
			}
		}
	}

	return "other"
}
