package handler

import "github.com/go-chi/chi/v5"

func NewRouter(
	orderHandler *OrderHandler,
	cartHandler *CartHandler,
) *chi.Mux {
	router := chi.NewRouter()

	router.Post("/orders", orderHandler.CreateOrder)
	router.Post("/cart/items", cartHandler.AddItem)
	router.Get("/cart", cartHandler.GetCart)
	router.Patch("/cart/items/{itemId}", cartHandler.UpdateItemQuantity)
	router.Delete("/cart/items/{itemId}", cartHandler.DeleteItem)
	router.Get("/orders/{orderId}", orderHandler.GetOrder)
	router.Post("/orders/{orderId}/pay", orderHandler.PayForTheOrder)

	return router
}
