package handler

import "github.com/go-chi/chi/v5"

func NewRouter(userHandler *UserHandler) *chi.Mux {
	router := chi.NewRouter()

	router.Post("/users/register", userHandler.Register)
	router.Post("/users/login", userHandler.Login)
	router.Get("/users/{userID}", userHandler.GetByID)

	return router
}
