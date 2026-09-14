package kafka

import "github.com/anuraghagawane/luma/internal/domain"

const (
	MAX_RETRIES        = 5
	INITIAL_BACKOFF_MS = 2000
	MAX_BACKOFF_MS     = 32000
)

func isRetryable(err error) bool {
	domainErr, ok := err.(*domain.LogError)
	if !ok {
		return true
	}

	return domainErr.Type == domain.ErrorTypeRetryable
}
