package nodes

import (
	"bytes"
	"capyflow/api/config"
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type TelegramNode struct{}

type TelegramParams struct {
	ChatID    string `json:"chatId"`
	Message   string `json:"message"`
	ParseMode string `json:"parseMode"` // Optional: "Markdown", "HTML", or empty
}

type TelegramResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"result"`
	Description string `json:"description"`
}

func (n *TelegramNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params TelegramParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error parsing telegram params: %v", err)
	}

	// Interpolate dynamic values from previous node outputs
	params.ChatID = interpolateString(params.ChatID, prev)
	params.Message = interpolateString(params.Message, prev)

	// Load global Telegram config
	cfg := config.LoadConfig()
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not configured")
	}

	// If chatId is empty, try to use default from config
	chatID := params.ChatID
	if chatID == "" {
		chatID = cfg.Telegram.DefaultChatID
	}

	if chatID == "" {
		return nil, fmt.Errorf("chatId is required (no default configured)")
	}

	if params.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	// Build Telegram API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	// Prepare request body
	requestBody := map[string]interface{}{
		"chat_id": chatID,
		"text":    params.Message,
	}

	if params.ParseMode != "" {
		requestBody["parse_mode"] = params.ParseMode
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Make HTTP request to Telegram API
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error sending telegram message: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Parse Telegram response
	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return nil, fmt.Errorf("error parsing telegram response: %v", err)
	}

	output := map[string]interface{}{
		"sent":    telegramResp.OK,
		"chatId":  chatID,
		"message": params.Message,
	}

	if telegramResp.OK {
		output["messageId"] = telegramResp.Result.MessageID
	} else {
		output["error"] = telegramResp.Description
		return output, fmt.Errorf("telegram API error: %s", telegramResp.Description)
	}

	return output, nil
}
