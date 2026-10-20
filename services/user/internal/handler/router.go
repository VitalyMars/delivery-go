package handler

import (
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"github.com/go-chi/chi/v5"
)

func NewRouter(userHandler *UserHandler) *chi.Mux {
	router := chi.NewRouter()

	router.Group(func(r chi.Router) {
		r.Use(transport.LimitAuth)
		r.Post("/users/register", userHandler.Register)
		r.Post("/users/login", userHandler.Login)
	})
	router.Get("/users/{userID}", userHandler.GetByID)

	return router
}
