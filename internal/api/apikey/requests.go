package apikey

import (
	"fmt"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/google/uuid"
)

type APIKeyPostBody struct {
	Name string `json:"name"`
}

func (b *APIKeyPostBody) validate() error {
	if len(b.Name) == 0 || len(b.Name) > 100 {
		return fmt.Errorf("invalid name")
	}

	return nil
}

func validateKeyID(id string) error {
	if err := uuid.Validate(id); err != nil {
		return fmt.Errorf("invalid id")
	}
	return nil
}

type APIKeyGetResponseObj struct {
	KeyID     string    `json:"key_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

func ResponseFromAPIKeys(keys []domain.APIKey) []APIKeyGetResponseObj {
	var response []APIKeyGetResponseObj

	for _, key := range keys {
		response = append(response, APIKeyGetResponseObj{KeyID: key.KeyID, Name: key.Name, Status: string(key.Status), CreatedAt: key.CreatedAt, UpdatedAt: key.UpdatedAt, Version: key.Version})
	}

	return response
}
