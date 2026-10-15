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
) error {
	const query = `
		INSERT INTO notifications (
			id, user_id, order_id, recipient_email,
			subject, body, status, attempts
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		n.ID,
		n.UserID,
		n.OrderID,
		n.RecipientEmail,
		n.Subject,
		n.Body,
		n.Status,
		n.Attempts,
	)
	return err
}

func (r *NotificationRepository) MarkSent(
	ctx context.Context,
	id uuid.UUID,
) error {
	const query = `
		UPDATE notifications
		SET status = 'sent',
			sent_at = NOW(),
			last_error = NULL
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) MarkFailed(
	ctx context.Context,
	id uuid.UUID,
	attempts int,
	lastError string,
) error {
	const query = `
		UPDATE notifications
		SET status = 'failed',
			attempts = $2,
			last_error = $3
		WHERE id = $1
	`

	tag, err := r.db.Exec(ctx, query, id, attempts, lastError)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*Notification, error) {
	const query = `
		SELECT
			id, user_id, order_id, recipient_email,
			subject, body, status, attempts,
			last_error, created_at, sent_at
		FROM notifications
		WHERE id = $1
	`

	n := &Notification{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&n.ID,
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
