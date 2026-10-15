package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")
var ErrEmailExists = errors.New("email already exists")

type User struct {
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string
}

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(
	ctx context.Context,
	user User,
) error {
	const query = `
		INSERT INTO users (id, name, email, password_hash)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		user.ID,
		user.Name,
		user.Email,
		user.PasswordHash,
	)
	if err != nil {
		var pgErr *pgx.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrEmailExists
		}
		return err
	}

	return nil
}

func (r *UserRepository) GetByEmail(
	ctx context.Context,
	email string,
) (User, error) {
	const query = `
		SELECT id, name, email, password_hash
		FROM users
		WHERE email = $1
	`

	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (r *UserRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (User, error) {
	const query = `
		SELECT id, name, email, password_hash
		FROM users
		WHERE id = $1
	`

	var user User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	return user, nil
}
