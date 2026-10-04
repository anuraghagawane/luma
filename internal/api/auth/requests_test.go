package auth

import "testing"

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name       string
		inputEmail string
		wantErr    bool
	}{
		{
			"valid",
			"abc@abc.ca",
			false,
		},
		{
			"invalid without @",
			"abcabc.ca",
			true,
		},
		{
			"invalid without .",
			"abc@abcca",
			true,
		},
		{
			"invalid without provider",
			"abc@.ca",
			true,
		},
		{
			"invalid without TLD",
			"abc@abc.",
			true,
		},
		{
			"invalid plain text",
			"abc",
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateEmail(test.inputEmail)
			if test.wantErr != (err != nil) {
				t.Errorf("validateEmail() error: %v, wantErr: %v, input: %v \n", err, test.wantErr, test.inputEmail)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name          string
		inputPassword string
		wantErr       bool
	}{
		{
			"valid",
			"abccde@fadfaA123",
			false,
		},
		{
			"invalid no digit",
			"abccde@fadfaA",
			true,
		},
		{
			"invalid no special character",
			"abccdefadfaA123",
			true,
		},
		{
			"invalid no capital letter",
			"abccde@fadfa123",
			true,
		},
		{
			"invalid no lower letter",
			"A@AFDFA123",
			true,
		},
		{
			"invalid min length",
			"abc",
			true,
		},
		{
			"invalid max length",
			"abcabcfdasfasfasdfasfasfaabcfdasfasfasdfasfasfaabcfdasfasfasdfasfasfafdasfasfasdfasfasfa",
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePassword(test.inputPassword)
			if test.wantErr != (err != nil) {
				t.Errorf("validatePassword() error: %v, wantErr: %v, input: %v \n", err, test.wantErr, test.inputPassword)
			}
		})
	}
}

func TestValidateTenantName(t *testing.T) {
	tests := []struct {
		name            string
		inputTenantName string
		wantErr         bool
	}{
		{
			"valid",
			"afadffadsfa",
			false,
		},
		{
			"valid with underscore",
			"afdasfa_fasdfa",
			false,
		},
		{
			"invalid number present",
			"abccd123",
			true,
		},
		{
			"invalid special character",
			"abccde@fadf",
			true,
		},
		{
			"invalid min length",
			"abc",
			true,
		},
		{
			"invalid max length",
			"abcabcfdasfasfasdfasfasfaabcfdasfasfasdfasfasfaabcfdasfasfasdfasfasfafdasfasfasdfasfasfa",
			true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTenantName(test.inputTenantName)
			if test.wantErr != (err != nil) {
				t.Errorf("validateTenantName() error: %v, wantErr: %v, input: %v \n", err, test.wantErr, test.inputTenantName)
			}
		})
	}
}
