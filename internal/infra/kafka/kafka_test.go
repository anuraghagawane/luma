package kafka

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		param    int
		expected time.Duration
	}{
		{0, 2000 * time.Millisecond},
		{1, 4000 * time.Millisecond},
		{2, 8000 * time.Millisecond},
		{10, 32000 * time.Millisecond},
	}

	for _, test := range tests {
		backoff := calculateBackoff(test.param)
		if backoff != test.expected {
			t.Errorf("Incorrect Backoff duration for attempt: %d, got: %d, expected: %d", test.param, backoff, test.expected)
		}
	}
}
