package apikey

import (
	"fmt"

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
