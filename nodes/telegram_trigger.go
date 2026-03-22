package nodes

import (
	"capyflow/api/config"
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type TelegramTriggerNode struct{}

// telegramGetFileResponse represents the response from Telegram getFile API
type telegramGetFileResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
		FileID   string `json:"file_id"`
		FilePath string `json:"file_path"`
	} `json:"result"`
}

func (n *TelegramTriggerNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	// Base output in case there is no webhook data
	baseOutput := map[string]interface{}{
		"triggered": true,
		"source":    "telegram",
		"timestamp": time.Now().Unix(),
	}

	webhookData, exists := context["webhook"]
	if !exists || webhookData == nil {
		return baseOutput, nil
	}

	// We expect Telegram to POST its update JSON body directly, so webhookData
	// should already be the update payload with additional metadata added in
	// the webhook handler (method, headers, etc.).

	// Try to extract the Telegram message
	msgAny, ok := webhookData["message"]
	if !ok {
		// Not a standard message update; just return raw payload
		baseOutput["raw"] = webhookData
		return baseOutput, nil
	}

	msg, ok := msgAny.(map[string]interface{})
	if !ok {
		baseOutput["raw"] = webhookData
		return baseOutput, nil
	}

	output := map[string]interface{}{
		"triggered": true,
		"source":    "telegram",
		"raw":       webhookData,
	}

	// Timestamp: try to use Telegram message.date (seconds) if present
	if ts, ok := msg["date"]; ok {
		output["timestamp"] = ts
	} else {
		output["timestamp"] = time.Now().Unix()
	}

	// Chat info
	if chatAny, ok := msg["chat"]; ok {
		if chat, ok := chatAny.(map[string]interface{}); ok {
			if id, ok := chat["id"]; ok {
				output["chatId"] = fmt.Sprintf("%v", id)
			}
			if title, ok := chat["title"].(string); ok {
				output["chatTitle"] = title
			}
			if username, ok := chat["username"].(string); ok {
				output["chatUsername"] = username
			}
		}
	}

	// Sender info
	if fromAny, ok := msg["from"]; ok {
		if from, ok := fromAny.(map[string]interface{}); ok {
			sender := map[string]interface{}{}
			if id, ok := from["id"]; ok {
				sender["id"] = fmt.Sprintf("%v", id)
			}
			if username, ok := from["username"].(string); ok {
				sender["username"] = username
			}
			if firstName, ok := from["first_name"].(string); ok {
				sender["first_name"] = firstName
			}
			if lastName, ok := from["last_name"].(string); ok {
				sender["last_name"] = lastName
			}
			output["from"] = sender
		}
	}

	// Text message
	if text, ok := msg["text"].(string); ok {
		output["text"] = text
	}

	// If a document (e.g., CSV/Excel) is attached, try to download it
	if docAny, ok := msg["document"]; ok {
		if doc, ok := docAny.(map[string]interface{}); ok {
			document := map[string]interface{}{}
			var fileID string

			if id, ok := doc["file_id"].(string); ok {
				document["file_id"] = id
				fileID = id
			}
			if fileName, ok := doc["file_name"].(string); ok {
				document["file_name"] = fileName
			}
			if mime, ok := doc["mime_type"].(string); ok {
				document["mime_type"] = mime
			}

			output["document"] = document

			// Try to download file content using global Telegram config
			if fileID != "" {
				if content, err := downloadTelegramFileContent(fileID); err == nil {
					// For CSV files, this content can be sent directly to the csv-parser node
					output["fileContent"] = content
				} else {
					output["fileDownloadError"] = err.Error()
				}
			}
		}
	}

	return output, nil
}

// downloadTelegramFileContent uses the global Telegram bot token to download the
// contents of a file (e.g., a CSV) given its file_id. This is best-effort; if
// anything fails, the caller can still use the raw document metadata.
func downloadTelegramFileContent(fileID string) (string, error) {
	cfg := config.LoadConfig()
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return "", fmt.Errorf("TELEGRAM_BOT_TOKEN is not configured")
	}

	// Step 1: get file path from Telegram
	getFileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, url.QueryEscape(fileID))
	resp, err := http.Get(getFileURL)
	if err != nil {
		return "", fmt.Errorf("error calling getFile: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading getFile response: %v", err)
	}

	var gfResp telegramGetFileResponse
	if err := json.Unmarshal(body, &gfResp); err != nil {
		return "", fmt.Errorf("error parsing getFile response: %v", err)
	}

	if !gfResp.OK {
		if gfResp.Description != "" {
			return "", fmt.Errorf("telegram getFile error: %s", gfResp.Description)
		}
		return "", fmt.Errorf("telegram getFile request failed")
	}

	if gfResp.Result.FilePath == "" {
		return "", fmt.Errorf("telegram getFile returned empty file_path")
	}

	// Step 2: download the actual file
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, gfResp.Result.FilePath)
	fileResp, err := http.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("error downloading file: %v", err)
	}
	defer fileResp.Body.Close()

	fileBytes, err := io.ReadAll(fileResp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading file content: %v", err)
	}

	// For CSV/text files this will be the actual content. For Excel (.xlsx)
	// you would typically add a specialized parser later.
	return string(fileBytes), nil
}
