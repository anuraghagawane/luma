package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestNewTokenManager(t *testing.T) {
	tests := []struct {
		name          string
		secret        string
		tokenLifetime int
		wantErr       bool
	}{
		{
			name:          "valid",
			secret:        "fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf",
			tokenLifetime: 2,
			wantErr:       false,
		},
		{
			name:          "negative life time",
			secret:        "fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf",
			tokenLifetime: -2,
			wantErr:       true,
		},
		{
			name:          "zero life time",
			secret:        "fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf",
			tokenLifetime: 0,
			wantErr:       true,
		},
		{
			name:          "short secret",
			secret:        "afdasfads",
			tokenLifetime: 4,
			wantErr:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokenManager, err := NewTokenManager(test.secret, test.tokenLifetime)

			if test.wantErr != (err != nil) {
				t.Errorf("NewTokenManager() error: %v, wantErr: %v, test_input: %+v", err, test.wantErr, test)
				return
			}

			if !test.wantErr && (string(tokenManager.key) != test.secret || tokenManager.tokenLifetime != test.tokenLifetime) {
				t.Errorf("NewTokenManager(%q), got: %v, want: %v", []any{test.secret, test.tokenLifetime}, []any{string(tokenManager.key), tokenManager.tokenLifetime}, []any{test.secret, test.tokenLifetime})
				return
			}
		})
	}
}

func TestIssueToken(t *testing.T) {
	tokenManager, err := NewTokenManager("fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf", 2)
	if err != nil {
		t.Errorf("error in NewTokenManager(), error: %v", err)
		return
	}
	user := domain.User{
		ID:           "12345",
		TenantID:     "12345",
		Email:        "abc@abc.abc",
		PasswordHash: "afdsfasdfa",
		Role:         domain.ADMIN,
		Status:       domain.ACTIVE,
	}

	before := time.Now()

	token, err := tokenManager.IssueToken(&user)
	if err != nil {
		t.Errorf("IssueToken(), error: %v, token: %v, user: %v", err, token, user)
		return
	}

	claims, err := tokenManager.ParseToken(token)
	if err != nil {
		t.Errorf("Error in parseToken(), error: %v", err)
		return
	}

	if claims.Email != user.Email {
		t.Errorf("Email mismatch, got: %v, wanted: %v", claims.Email, user.Email)
		return
	}

	if claims.ID != user.ID {
		t.Errorf("ID mismatch, got: %v, wanted: %v", claims.ID, user.ID)
		return
	}

	if claims.TenantID != user.TenantID {
		t.Errorf("TenantID mismatch, got: %v, wanted: %v", claims.TenantID, user.TenantID)
		return
	}

	if claims.Role != string(user.Role) {
		t.Errorf("Role mismatch, got: %v, wanted: %v", claims.Role, user.Role)
		return
	}

	currTime := time.Now()
	if !claims.ExpiresAt.After(before.Add(119 * time.Minute)) {
		t.Errorf("ExpireAt corrupted, expected time after 1 hour, expiry time: %v, current time: %v", claims.ExpiresAt, before)
		return
	}

	if claims.IssuedAt.After(currTime) {
		t.Errorf("IssuedAt corrupted, expected time before current time, issued time: %v, current time: %v", claims.IssuedAt, currTime)
		return
	}

	if claims.Issuer != "luma" {
		t.Errorf("Issuer mismatch, got: %v, expected: %v", claims.Issuer, "luma")
		return
	}
}

func TestParseToken(t *testing.T) {
	user := domain.User{
		ID:           "12345",
		TenantID:     "12345",
		Email:        "abc@abc.abc",
		PasswordHash: "afdsfasdfa",
		Role:         domain.ADMIN,
		Status:       domain.ACTIVE,
	}

	key := "fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfas"
	tokenManager, err := NewTokenManager(key, 2)
	if err != nil {
		t.Errorf("error in NewTokenManager(), error: %v", err)
		return
	}
	tests := []struct {
		name        string
		tokenString string
		wantErr     bool
	}{
		{
			name:        "valid",
			tokenString: generateToken(key, 2, user, jwt.SigningMethodHS256, false, "luma", false),
			wantErr:     false,
		},
		{
			name:        "invalid",
			tokenString: "not a key",
			wantErr:     true,
		},
		{
			name:        "wrong signature",
			tokenString: generateToken("fafasfwerqwerqtriouqyweoyqoihfsajhfalkh", 2, user, jwt.SigningMethodHS256, false, "luma", false),
			wantErr:     true,
		},
		{
			name:        "wrong issuer",
			tokenString: generateToken(key, 2, user, jwt.SigningMethodHS256, false, "acdfeq", false),
			wantErr:     true,
		},
		{
			name:        "wrong signing method",
			tokenString: generateToken(key, 2, user, jwt.SigningMethodHS384, false, "luma", false),
			wantErr:     true,
		},
		{
			name:        "expired token",
			tokenString: generateToken(key, 2, user, jwt.SigningMethodHS256, true, "luma", false),
			wantErr:     true,
		},
		{
			name:        "no expiration",
			tokenString: generateToken(key, 2, user, jwt.SigningMethodHS256, false, "luma", true),
			wantErr:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := tokenManager.ParseToken(test.tokenString)
			if test.wantErr != (err != nil) {
				t.Errorf("ParseToken(), err: %v, wantErr: %v, input: %v", err, test.wantErr, test.tokenString)
			}
		})
	}
}

func generateToken(key string, tokenLifetime int, user domain.User, algo jwt.SigningMethod, issueExpired bool, issuer string, noExpiry bool) string {
	m, err := NewTokenManager(key, tokenLifetime)
	if err != nil {
		fmt.Printf("error in generateToken: %v", err)
		return ""
	}
	now := time.Now()
	lifeTime := m.tokenLifetime
	if issueExpired {
		lifeTime = -m.tokenLifetime
	}
	expirationTime := now.Add(time.Hour * time.Duration(lifeTime))
	claims := &CustomClaims{
		ID:       user.ID,
		Email:    user.Email,
		TenantID: user.TenantID,
		Role:     string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    issuer,
		},
	}

	if noExpiry {
		claims.ExpiresAt = nil
	}

	token := jwt.NewWithClaims(algo, claims)

	tokenString, err := token.SignedString(m.key)
	if err != nil {
		fmt.Printf("error in generateToken: %v", err)
		return ""
	}

	return tokenString
}
