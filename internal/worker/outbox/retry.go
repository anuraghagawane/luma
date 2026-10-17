package outbox

import (
	"math/rand/v2"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

func (p RetryPolicy) Delay(attempt int) time.Duration {
	capDelay := p.BaseDelay
	for i := 1; i < attempt && capDelay < p.MaxDelay; i++ {
		if capDelay > p.MaxDelay/2 {
			capDelay = p.MaxDelay
			break
		}
		capDelay *= 2
	}

	if capDelay > p.MaxDelay {
		capDelay = p.MaxDelay
	}
	if capDelay <= 0 {
		return 0
	}

	return time.Duration(rand.Int64N(int64(capDelay)))
}
