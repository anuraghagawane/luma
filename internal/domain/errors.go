package domain

type ErrorType int

const (
	ErrorTypeRetryable ErrorType = iota
	ErrorTypePermanent
)

type LogError struct {
	Type    ErrorType
	Message string
	Code    int
}

func (e LogError) Error() string {
	return e.Message
}
