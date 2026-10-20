package handler

import "github.com/go-chi/chi/v5"

func NewRouter(
	orderHandler *OrderHandler,
) *chi.Mux {
	router := chi.NewRouter()

	router.Post("/orders", orderHandler.CreateOrder)
	router.Get("/orders/{orderId}", orderHandler.GetOrder)
	router.Post("/orders/{orderId}/pay", orderHandler.PayForTheOrder)

	return router
}
