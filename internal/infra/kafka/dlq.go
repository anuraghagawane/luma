package kafka

import (
	"encoding/json"
	"time"

	"github.com/anuraghagawane/luma/internal/domain"
)

type DLQMessage struct {
	Log       domain.Log `json:"log"`
	Error     string     `json:"error"`
	ErrorType string     `json:"error_type"`
	Attempts  int        `json:"attempts"`
	Timestamp int64      `json:"timestamp"`
}

func NewDLQMessage(log domain.Log, err error, errorType string, attempts int) *DLQMessage {
	return &DLQMessage{
		Log:       log,
		Error:     err.Error(),
		ErrorType: errorType,
		Attempts:  attempts,
		Timestamp: time.Now().Unix(),
	}
}

func (d *DLQMessage) ToJSON() ([]byte, error) {
	return json.Marshal(d)
}
