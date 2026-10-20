package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("notification not found")

type Notification struct {
	ID             uuid.UUID
	EventID        string
	UserID         uuid.UUID
	OrderID        uuid.UUID
	RecipientEmail string
	Subject        string
	Body           string
	Status         string
	Attempts       int
	LastError      *string
	CreatedAt      time.Time
	SentAt         *time.Time
}

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(
	ctx context.Context,
	n *Notification,
) (bool, error) {
	const query = `
        INSERT INTO notifications (
            id, event_id, user_id, order_id, recipient_email,
            subject, body, status, attempts
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (event_id) WHERE event_id IS NOT NULL
        DO NOTHING
    `

	tag, err := r.db.Exec(
		ctx,
		query,
		n.ID,
		n.EventID,
		n.UserID,
		n.OrderID,
		n.RecipientEmail,
		n.Subject,
		n.Body,
		n.Status,
		n.Attempts,
	)
	if err != nil {
		return false, err
	}

	return tag.RowsAffected() == 1, nil
}

func (r *NotificationRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*Notification, error) {
	const query = `
SELECT
    id, event_id, user_id, order_id, recipient_email,
    subject, body, status, attempts,
    last_error, created_at, sent_at
FROM notifications
WHERE id = $1
	`

	n := &Notification{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&n.ID,
		&n.EventID,
		&n.UserID,
		&n.OrderID,
		&n.RecipientEmail,
		&n.Subject,
		&n.Body,
		&n.Status,
		&n.Attempts,
		&n.LastError,
		&n.CreatedAt,
		&n.SentAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return n, nil
}

// DeliverNext locks one due notification throughout its bounded SMTP attempt.
// A crashed worker releases the lock by rolling back, making the task eligible again.
func (r *NotificationRepository) DeliverNext(ctx context.Context, send func(context.Context, string, string, string) error) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	var n Notification
	err = tx.QueryRow(ctx, `SELECT id,recipient_email,subject,body,attempts FROM notifications
 WHERE status IN ('pending','failed') AND attempts < 10 AND next_attempt_at <= now()
 ORDER BY next_attempt_at,created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&n.ID, &n.RecipientEmail, &n.Subject, &n.Body, &n.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sendErr := send(ctx, n.RecipientEmail, n.Subject, n.Body)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	n.Attempts++
	if sendErr == nil {
		_, err = tx.Exec(ctx, "UPDATE notifications SET status='sent',sent_at=now(),last_error=NULL,attempts=$2 WHERE id=$1", n.ID, n.Attempts)
	} else {
		delay := time.Second * time.Duration(1<<min(n.Attempts, 10))
		_, err = tx.Exec(ctx, "UPDATE notifications SET status='failed',last_error=$2,attempts=$3,next_attempt_at=$4 WHERE id=$1", n.ID, sendErr.Error(), n.Attempts, time.Now().Add(delay))
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
