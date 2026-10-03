package auth

import (
	"errors"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

type TokenManager struct {
	key []byte
}

func NewTokenManager(secret string) (*TokenManager, error) {
	if len(secret) < 5 {
		return nil, errors.New("invalid jwt secret, need secret atleast 5 characters long")
	}
	return &TokenManager{[]byte(secret)}, nil
}

type customClaims struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	TenantID string `json:"tenantid"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (m *TokenManager) IssueToken(user *domain.User) (string, error) {
	expirationTime := time.Now().Add(1 * time.Minute)

	claims := &customClaims{
		ID:       user.ID,
		Email:    user.Email,
		TenantID: user.TenantID,
		Role:     string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "luma",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(m.key)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
