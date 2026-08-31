package domain

import "testing"

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
