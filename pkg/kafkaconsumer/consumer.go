// Package kafkaconsumer provides ordered, explicitly committed event processing.
package kafkaconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type permanentError struct{ error }

type Client interface {
	PollRecords(context.Context, int) kgo.Fetches
	ProduceSync(context.Context, ...*kgo.Record) kgo.ProduceResults
	CommitRecords(context.Context, ...*kgo.Record) error
	AllowRebalance()
	CloseAllowingRebalance()
}

// Permanent marks malformed input. It is preserved in a DLQ before committing.
func Permanent(err error) error { return permanentError{err} }

// Run stops on exhausted transient errors: a supervisor may safely restart it.
// It never advances past a failed record. Side effects must still be idempotent.
func Run(ctx context.Context, client Client, handle func(context.Context, *kgo.Record) error) error {
	defer client.CloseAllowingRebalance()
	for ctx.Err() == nil {
		fetches := client.PollRecords(ctx, 1)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) != 0 {
			return fmt.Errorf("Kafka fetch: %w", errs[0].Err)
		}
		for _, record := range fetches.Records() {
			if err := retry(ctx, func(attemptCtx context.Context) error { return handle(attemptCtx, record) }); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				var invalid permanentError
				if !errors.As(err, &invalid) {
					return fmt.Errorf("processing %s[%d]@%d: %w", record.Topic, record.Partition, record.Offset, err)
				}
				payload, marshalErr := json.Marshal(struct {
					Topic     string `json:"topic"`
					Partition int32  `json:"partition"`
					Offset    int64  `json:"offset"`
					Value     []byte `json:"value_base64"`
					Error     string `json:"error"`
				}{record.Topic, record.Partition, record.Offset, record.Value, err.Error()})
				if marshalErr != nil {
					return marshalErr
				}
				dlqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				dlqErr := client.ProduceSync(dlqCtx, &kgo.Record{Topic: record.Topic + ".dlq", Key: record.Key, Value: payload}).FirstErr()
				cancel()
				if dlqErr != nil {
					return fmt.Errorf("publish DLQ: %w", dlqErr)
				}
				log.Printf("event quarantined: topic=%s partition=%d offset=%d", record.Topic, record.Partition, record.Offset)
			}
			commitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := client.CommitRecords(commitCtx, record)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("commit record: %w", err)
			}
		}
		client.AllowRebalance()
	}
	return nil
}

func retry(ctx context.Context, action func(context.Context) error) error {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		last = action(attemptCtx)
		cancel()
		if last == nil {
			return nil
		}
		var invalid permanentError
		if errors.As(last, &invalid) || ctx.Err() != nil {
			return last
		}
		if attempt == 3 {
			break
		}
		timer := time.NewTimer(time.Second << attempt)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}
