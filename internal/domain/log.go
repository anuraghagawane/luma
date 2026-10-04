// Package domain lists out all the core domain/business logics of appliation
package domain

import (
	"context"
	"fmt"
	"time"
)

type LogLevel string

const (
	ERROR LogLevel = "ERROR"
	DEBUG LogLevel = "DEBUG"
)

func (loglevel *LogLevel) UnmarshalText(text []byte) error {
	str := LogLevel(text)
	switch str {
	case ERROR, DEBUG:
		*loglevel = str
		return nil
	default:
		return fmt.Errorf("invalid loglevel: %s", string(text))
	}
}

type Log struct {
	EventID   string   `json:"eventid"`
	Tenant    string   `json:"tenant"`
	Service   string   `json:"service"`
	Host      string   `json:"host"`
	LogLevel  LogLevel `json:"loglevel"`
	Message   string   `json:"message"`
	Timestamp int64    `json:"timestamp"`
}

type LogQuery struct {
	Tenant   string   `json:"-"`
	From     int64    `json:"from"`
	To       int64    `json:"to"`
	Service  string   `json:"service"`
	LogLevel LogLevel `json:"log_level"`
	Keyword  string   `json:"keyword"`
	Cursor   string   `json:"cursor"`
	Limit    int64    `json:"limit"`
}

func (q *LogQuery) ValidateAndDefault() error {
	if q.Tenant == "" {
		return fmt.Errorf("invalid tenant")
	}

	if q.Limit > 1000 || q.Limit < 0 {
		return fmt.Errorf("invalid input max limit is 1000")
	}

	if q.To <= q.From {
		return fmt.Errorf("invalid input timestamp")
	}

	if q.From < time.Now().AddDate(0, 0, -30).Unix() {
		return fmt.Errorf("invalid input, can query only recent 30 days data")
	}

	if q.To > time.Now().Unix() {
		return fmt.Errorf("invalid input cannot query future data")
	}

	if q.Limit == 0 {
		q.Limit = 1000
	}

	return nil
}

type QueryResponse struct {
	Cursor *string `json:"cursor"`
	Logs   []Log   `json:"logs"`
}
type LogProducer interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte) error
}

type LogRepository interface {
	Index(ctx context.Context, id string, document Log) error
}

type QueryRepository interface {
	Query(ctx context.Context, logQuery LogQuery) (*QueryResponse, error)
}
