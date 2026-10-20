package kafkaconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeClient struct {
	records    []*kgo.Record
	committed  []int64
	dlq        []*kgo.Record
	publishErr error
	cancel     context.CancelFunc
}

func (c *fakeClient) PollRecords(context.Context, int) kgo.Fetches {
	if len(c.records) == 0 {
		c.cancel()
		return nil
	}
	record := c.records[0]
	c.records = c.records[1:]
	return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: record.Topic, Partitions: []kgo.FetchPartition{{Records: []*kgo.Record{record}}}}}}}
}
func (c *fakeClient) CommitRecords(_ context.Context, records ...*kgo.Record) error {
	for _, r := range records {
		c.committed = append(c.committed, r.Offset)
	}
	return nil
}
func (c *fakeClient) ProduceSync(_ context.Context, records ...*kgo.Record) kgo.ProduceResults {
	c.dlq = append(c.dlq, records...)
	return kgo.ProduceResults{{Record: records[0], Err: c.publishErr}}
}
func (*fakeClient) AllowRebalance()         {}
func (*fakeClient) CloseAllowingRebalance() {}

func TestTransientFailureDoesNotCommitOrSkip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &fakeClient{records: []*kgo.Record{{Topic: "orders", Offset: 10}, {Topic: "orders", Offset: 11}}, cancel: cancel}
	calls := 0
	err := Run(ctx, client, func(_ context.Context, r *kgo.Record) error {
		calls++
		if r.Offset != 10 {
			t.Fatal("advanced past failed record")
		}
		return errors.New("database unavailable")
	})
	if err == nil || calls != 4 || len(client.committed) != 0 || len(client.records) != 1 {
		t.Fatalf("err=%v calls=%d committed=%v remaining=%d", err, calls, client.committed, len(client.records))
	}
}

func TestPermanentErrorCommitsOnlyAfterDurableDLQ(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "published", true: "DLQ unavailable"}[fail], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &fakeClient{records: []*kgo.Record{{Topic: "orders", Offset: 10, Value: []byte("broken JSON")}}, cancel: cancel}
			if fail {
				client.publishErr = errors.New("broker unavailable")
			}
			err := Run(ctx, client, func(context.Context, *kgo.Record) error { return Permanent(errors.New("bad input")) })
			if fail {
				if err == nil || len(client.committed) != 0 {
					t.Fatal("committed without DLQ")
				}
			} else {
				if err != nil || len(client.committed) != 1 {
					t.Fatalf("err=%v commits=%v", err, client.committed)
				}
				var message struct {
					Value []byte `json:"value_base64"`
				}
				if json.Unmarshal(client.dlq[0].Value, &message) != nil || string(message.Value) != "broken JSON" || client.dlq[0].Topic != "orders.dlq" {
					t.Fatal("DLQ lost original event")
				}
			}
		})
	}
}
