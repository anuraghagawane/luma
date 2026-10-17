package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/anuraghagawane/luma/internal/domain"
)

func generateKeyObj(name string, tenantID string, userID string) (*domain.APIKey, error) {
	id := uuid.New()

	key, err := generateKey(id.String())
	if err != nil {
		return nil, fmt.Errorf("error while generating key")
	}

	hash := generateHash(key)

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
		return "", fmt.Errorf("error creating the hash")
	}

	randomByteBase64 := base64.StdEncoding.EncodeToString(randomBytes)
	return "luma_" + keyID + "_" + randomByteBase64, nil
}

func generateHash(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hashSum := hasher.Sum(nil)

	return hex.EncodeToString(hashSum)
}
