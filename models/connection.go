package models

import (
	"capyflow/api/helpers"
	"encoding/json"
	"fmt"
	"time"
)

type Connection struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"userId" gorm:"index"`
	Name             string    `json:"name"`
	Provider         string    `json:"provider" gorm:"index"`
	SecretsEncrypted string    `json:"-" gorm:"type:text"`
	Metadata         string    `json:"metadata,omitempty" gorm:"type:text"`
	IsActive         bool      `json:"isActive" gorm:"default:true"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (Connection) TableName() string {
	return "connections"
}

func (c *Connection) SetSecrets(secrets map[string]interface{}) error {
	if secrets == nil {
		c.SecretsEncrypted = ""
		return nil
	}

	payload, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %v", err)
	}

	encrypted, err := helpers.EncryptString(string(payload))
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %v", err)
	}

	c.SecretsEncrypted = encrypted
	return nil
}

func (c *Connection) GetSecrets() (map[string]interface{}, error) {
	result := map[string]interface{}{}
	if c.SecretsEncrypted == "" {
		return result, nil
	}

	decrypted, err := helpers.DecryptString(c.SecretsEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets: %v", err)
	}

	if decrypted == "" {
		return result, nil
	}

	if err := json.Unmarshal([]byte(decrypted), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %v", err)
	}

	return result, nil
}
