package apikey

import "fmt"

type APIKeyPostBody struct {
	Name string `json:"name"`
}

func (b *APIKeyPostBody) validate() error {
	if len(b.Name) == 0 || len(b.Name) > 100 {
		return fmt.Errorf("invalid name")
	}

	return nil
}
