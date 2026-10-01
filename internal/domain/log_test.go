package domain

import (
	"testing"
	"time"
)

func TestLogLevelUnmarshal(t *testing.T) {
	var level LogLevel

	tests := []struct {
		level   []byte
		wantErr bool
	}{
		{[]byte("ERROR"), false},
		{[]byte("DEBUG"), false},
		{[]byte("ERROr"), true},
		{[]byte("WARN"), true},
	}

	for _, test := range tests {
		t.Run(string(test.level), func(t *testing.T) {
			err := level.UnmarshalText(test.level)
			if (err != nil) != test.wantErr {
				t.Errorf("Failed to unmarshal level=%v, wantErr=%v", string(test.level), test.wantErr)
			}
		})
	}
}

func TestLogQueryValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   LogQuery
		wantErr bool
	}{
		{"valid query", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", 500}, false},
		{"missing tenant", LogQuery{"", time.Now().AddDate(0, 0, -2).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", 500}, true},
		{"from before 30 days", LogQuery{"abc", time.Now().AddDate(0, 0, -40).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", 500}, true},
		{"future date", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().AddDate(0, 0, 2).Unix(), "abc", "DEBUG", "", "", 500}, true},
		{"to less tha from time", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().AddDate(0, 0, -4).Unix(), "abc", "DEBUG", "", "", 500}, true},
		{"negative limit", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", -1}, true},
		{"limit over max value", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", 1001}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.input.ValidateAndDefault(); (err != nil) != test.wantErr {
				t.Errorf("ValidateAndDefault() error \"%v\", ExpectErr: %v, LogQuery: %+v", err, test.wantErr, test.input)
			}
		})
	}
}

func TestLogQueryDefault(t *testing.T) {
	tests := []struct {
		name            string
		input           LogQuery
		changeToDefault bool
	}{
		{"no change", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", 5}, false},
		{"set to default", LogQuery{"abc", time.Now().AddDate(0, 0, -2).Unix(), time.Now().Unix(), "abc", "DEBUG", "", "", 0}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limitBeforeValidation := test.input.Limit

			if err := test.input.ValidateAndDefault(); err != nil {
				t.Fatalf("ValidateAndDefault() unexpected error: %v", err)
			}

			if test.changeToDefault && test.input.Limit != 1000 {
				t.Errorf("Expected limit value change to default, limitBefore: %v, limitAfter: %v", limitBeforeValidation, test.input.Limit)
			}

			if !test.changeToDefault && limitBeforeValidation != test.input.Limit {
				t.Errorf("Expected no limit value change, limitBefore: %v, limitAfter: %v", limitBeforeValidation, test.input.Limit)
			}
		})
	}
}
