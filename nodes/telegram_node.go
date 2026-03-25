package nodes

import (
	"bytes"
	"capyflow/api/config"
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	defaultChatID := strings.TrimSpace(cfg.Telegram.DefaultChatID)

	// If chatId is empty or unresolved placeholder, use default from config.
	chatID := strings.TrimSpace(params.ChatID)
	if chatID == "" || strings.Contains(chatID, "{{") || strings.Contains(chatID, "}}") {
		chatID = defaultChatID
	}

	if chatID == "" {
		return nil, fmt.Errorf("chatId is required (no default configured)")
	}

	if params.Message == "" {
		return nil, fmt.Errorf("message is required")
	}

	telegramResp, err := sendTelegramMessage(botToken, chatID, params.Message, params.ParseMode)
	if err != nil {
		return nil, err
	}

	// Retry once with default chat id when node-level chat id is invalid.
	if !telegramResp.OK && strings.Contains(strings.ToLower(telegramResp.Description), "chat not found") {
		if defaultChatID != "" && defaultChatID != chatID {
			telegramRespRetry, retryErr := sendTelegramMessage(botToken, defaultChatID, params.Message, params.ParseMode)
			if retryErr != nil {
				return nil, retryErr
			}
			if telegramRespRetry.OK {
				return map[string]interface{}{
					"sent":                 true,
					"chatId":               defaultChatID,
					"message":              params.Message,
					"messageId":            telegramRespRetry.Result.MessageID,
					"retryWithDefaultChat": true,
				}, nil
			}
		}
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
		return output, fmt.Errorf("telegram API error (chatId=%s): %s", chatID, telegramResp.Description)
	}

	return output, nil
}

func sendTelegramMessage(botToken, chatID, message, parseMode string) (*TelegramResponse, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	requestBody := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
	}

	if parseMode != "" {
		requestBody["parse_mode"] = parseMode
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error sending telegram message: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var telegramResp TelegramResponse
	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return nil, fmt.Errorf("error parsing telegram response: %v", err)
	}

	return &telegramResp, nil
}
