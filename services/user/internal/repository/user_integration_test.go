package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/VitalyMars/delivery_go/pkg/testdb"
	"github.com/google/uuid"
)

func TestPGXErrorMapping(t *testing.T) {
	db := testdb.Open(t, "../../migrations")
	repo := NewUserRepository(db)
	ctx := context.Background()
	if _, err := repo.GetByEmail(ctx, "missing@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	user := User{ID: uuid.New(), Name: "User", Email: "user@example.com", PasswordHash: "hash"}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	user.ID = uuid.New()
	if err := repo.Create(ctx, user); !errors.Is(err, ErrEmailExists) {
		t.Fatal(err)
	}
}
