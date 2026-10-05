package auth

import "testing"

func TestPasswordHash(t *testing.T) {
	password := "afasdfadfa"
	hash, err := hashPassword(password)
	if err != nil {
		t.Errorf("hashPassword(), error: %v, expected: no error", err)
		return
	}

	if len(hash) == 0 {
		t.Errorf("hashPassword(), error: hash is empty")
		return
	}

	if hash == password {
		t.Errorf("hashPassword(), error: hash == password")
		return
	}
}

func TestComparePassword(t *testing.T) {
	password := "afasdfadfa"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("error while hashing password: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		wantErr  bool
	}{
		{
			name:     "valid",
			password: password,
			hash:     hash,
			wantErr:  false,
		},
		{
			name:     "wrong password",
			password: "afadfasf",
			hash:     hash,
			wantErr:  true,
		},
		{
			name:     "invalid hash",
			password: "afadfasf",
			hash:     "oaphoahfal;kjfla;k",
			wantErr:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := comparePassword(test.hash, test.password)
			if test.wantErr != (err != nil) {
				t.Errorf("comparePassword(), error: %v, wantErr: %v", err, test.wantErr)
			}
		})
	}
}
