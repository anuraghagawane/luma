// Package domain lists out all the core domain/business logics of appliation
package domain

import (
	"context"
	"fmt"
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
	Host      string   `json:"host"`
	LogLevel  LogLevel `json:"loglevel"`
	Message   string   `json:"message"`
	Timestamp int64    `json:"timestamp"`
}

type LogProducer interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte) error
}

type LogRepository interface {
	Index(ctx context.Context, id string, document Log) error
}
