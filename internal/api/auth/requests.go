package auth

import (
	"errors"
	"fmt"
	"regexp"
	"unicode"
)

type AccountCreateRequestBody struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantName string `json:"tenantname"`
}

func (b AccountCreateRequestBody) Validate() error {
	if err := validateEmail(b.Email); err != nil {
		return fmt.Errorf("invalid email")
	}

	if err := validatePassword(b.Password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	if err := validateTenantName(b.TenantName); err != nil {
		return fmt.Errorf("invalid tenant name: %w", err)
	}

	return nil
}

type LoginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (b LoginRequestBody) Validate() error {
	if err := validateEmail(b.Email); err != nil {
		return fmt.Errorf("invalid email")
	}

	if err := validatePassword(b.Password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	return nil
}

func validateEmail(email string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email")
	}
	return nil
}

func validatePassword(password string) error {
	var (
		hasMinLen  = len(password) >= 8
		hasMaxLen  = len(password) <= 64
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	if !hasMinLen || !hasMaxLen {
		return errors.New("password must be between 8 and 64 characters long")
	}

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain atleast one upper case character")
	}
	if !hasLower {
		return errors.New("password must contain atleast one lower case character")
	}
	if !hasNumber {
		return errors.New("password must contain atleast one digit character")
	}
	if !hasSpecial {
		return errors.New("password must contain atleast one special character(punctuation or symbol")
	}

	return nil
}

func validateTenantName(tenantName string) error {
	var (
		hasMinLen               = len(tenantName) >= 5
		hasMaxLen               = len(tenantName) <= 20
		hasNumber               bool
		hasSpecialNotUnderScore bool
	)

	if !hasMinLen || !hasMaxLen {
		return errors.New("tenant name must be between 5 and 20 characters long")
	}

	for _, char := range tenantName {
		switch {
		case unicode.IsNumber(char):
			hasNumber = true
		case (unicode.IsPunct(char) || unicode.IsSymbol(char)) && char != '_':
			hasSpecialNotUnderScore = true
		}
	}

	if hasNumber {
		return errors.New("tenant name should not contain the digit/number")
	}

	if hasSpecialNotUnderScore {
		return errors.New("tenant name should not contain special characters apart from underscore \"_\" ")
	}

	return nil
}
