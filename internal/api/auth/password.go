package auth

import "golang.org/x/crypto/bcrypt"

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func comparePassword(passwordHash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
}
