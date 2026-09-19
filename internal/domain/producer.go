package domain

import "context"

type EventProducer interface {
	Publish(ctx context.Context, topic string, key []byte, value []byte) error
	Close() error
}
