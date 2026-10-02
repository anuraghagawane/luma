package domain

import "context"

type UserRole string

const (
	ADMIN UserRole = "ADMIN"
	USER  UserRole = "USER"
)

type UserStatus string

const (
	ACTIVE    UserStatus = "ACTIVE"
	SUSPENDED UserStatus = "SUSPENDED"
	DELETED   UserStatus = "DELETED"
)

type User struct {
	ID           string     `json:"id"`
	TenantID     string     `json:"tenantid"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Role         UserRole   `json:"role"`
	Status       UserStatus `json:"status"`
}

type UserRepository interface {
	// Create(ctx context.Context, user *User) error
	CreateTenantAndUser(ctx context.Context, user *User, tenant *Tenant) error
}
