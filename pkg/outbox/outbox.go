// Package outbox serializes publishers using a transaction-scoped advisory lock.
package outbox

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Publish func(context.Context, string, string, any) error

// Batch keeps the lock and the database transaction until Kafka acknowledges.
// A crash after the broker ack can cause redelivery; consumers must deduplicate.
func Batch(ctx context.Context, db *pgxpool.Pool, publish Publish) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var locked bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(728105)`).Scan(&locked); err != nil || !locked {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, aggregate_id::text, event_type, payload FROM outbox_events WHERE published_at IS NULL ORDER BY created_at, id LIMIT 100 FOR UPDATE`)
	if err != nil {
		return err
	}
	type event struct {
		id, aggregate, topic string
		payload              []byte
	}
	var events []event
	for rows.Next() {
		var e event
		if err = rows.Scan(&e.id, &e.aggregate, &e.topic, &e.payload); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		if err = publish(ctx, e.topic, e.aggregate, json.RawMessage(e.payload)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE outbox_events SET published_at = now() WHERE id = $1`, e.id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
