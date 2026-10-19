// Package apikey carries api key utilities
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/google/uuid"
)

func GenerateAPIKey(name string, tenantID string, userID string) (*domain.APIKey, error) {
	id := uuid.New()

	key, err := generateKey(id.String())
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	hash := hashAPIKey(key)

	return &domain.APIKey{
		KeyID:     id.String(),
		Name:      name,
		TenantID:  tenantID,
		Key:       key,
		Hash:      hash,
		Version:   1,
		Status:    domain.APIKeyActive,
		CreatedBy: userID,
		UpdatedBy: userID,
	}, nil
}

func generateKey(keyID string) (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	randomByteBase64 := base64.StdEncoding.EncodeToString(randomBytes)
	return "luma_" + keyID + "_" + randomByteBase64, nil
}

func hashAPIKey(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hashSum := hasher.Sum(nil)

	return hex.EncodeToString(hashSum)
}

type Key struct {
	ID   string
	Hash string
}

func ValidateAndParseAPIKey(apikeyString string) (*Key, error) {
	parts := strings.Split(apikeyString, "_")
	if len(parts) != 3 {
		return nil, errors.New("apikey components count does not match")
	}

	if parts[0] != "luma" {
		return nil, errors.New("not a luma key")
	}

	if err := uuid.Validate(parts[1]); err != nil {
		return nil, errors.New("invalid key id")
	}

	if len(parts[2]) < 32 {
		return nil, errors.New("invalid key")
	}

	hash := hashAPIKey(apikeyString)

	return &Key{parts[1], hash}, nil
}
