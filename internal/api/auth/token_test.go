package auth

import "testing"

func TestNewTokenManager(t *testing.T) {
	tests := []struct {
		name          string
		secret        string
		tokenLifetime int
		wantErr       bool
	}{
		{
			"valid",
			"fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf",
			2,
			false,
		},
		{
			"negative life time",
			"fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf",
			-2,
			true,
		},
		{
			"zero life time",
			"fasfadfasfasdfasddfasdfasfasfasfasdfasdfasdfasdfasdfasdf",
			0,
			true,
		},
		{
			"short secret",
			"afdasfads",
			4,
			true,
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
