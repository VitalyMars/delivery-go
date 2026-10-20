package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/VitalyMars/delivery_go/services/user/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid email or password")
var ErrInvalidInput = errors.New("invalid registration input")

type UserService struct {
	repo *repository.UserRepository
}

type PublicUser struct {
	ID    uuid.UUID
	Name  string
	Email string
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Register(
	ctx context.Context,
	name string,
	email string,
	password string,
) (PublicUser, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" || email == "" || password == "" {
		return PublicUser{}, ErrInvalidInput
	}

	if len(password) < 8 || len(password) > 72 || len(name) > 100 || len(email) > 254 {
		return PublicUser{}, fmt.Errorf("%w: name <= 100 bytes, email <= 254 bytes, password 8..72 bytes", ErrInvalidInput)
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return PublicUser{}, ErrInvalidInput
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return PublicUser{}, err
	}

	user := repository.User{
		ID:           uuid.New(),
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return PublicUser{}, err
	}

	return PublicUser{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (s *UserService) Login(
	ctx context.Context,
	email string,
	password string,
) (PublicUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return PublicUser{}, ErrInvalidCredentials
		}
		return PublicUser{}, err
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	); err != nil {
		return PublicUser{}, ErrInvalidCredentials
	}

	return PublicUser{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}

func (s *UserService) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (PublicUser, error) {

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return PublicUser{}, err
	}

	return PublicUser{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}, nil
}
