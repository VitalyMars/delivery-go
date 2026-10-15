package handler

import (
	"github.com/go-chi/chi/v5"
)

func NewRouter(
	restaurantHandler *RestaurantHandler,
	dishHandler *DishHandler,
) *chi.Mux {
	router := chi.NewRouter()

	router.Get("/restaurants", restaurantHandler.GetRestaurants)
	router.Get("/restaurants/{restaurantId}", restaurantHandler.GetRestaurantByID)
	router.Get("/restaurants/{restaurantId}/menu", dishHandler.GetDishesByRestaurantID)
	router.Get("/dishes/{dishId}", dishHandler.GetDishByID)

	return router
}
