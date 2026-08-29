package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

type FranzProducer struct {
	client *kgo.Client
}

func NewFranzProducer(brokers []string) (*FranzProducer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	return &FranzProducer{
		client: cl,
	}, nil
}

func (fp *FranzProducer) Publish(ctx context.Context, topic string, key []byte, value []byte) error {
	record := &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	}

	err := fp.client.ProduceSync(ctx, record).FirstErr()
	if err != nil {
		return fmt.Errorf("kafka publish failed: %w", err)
	}

	return nil
}

func (fp *FranzProducer) Close() error {
	fp.client.Close()
	return nil
}
