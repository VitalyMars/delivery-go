package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VitalyMars/delivery_go/services/catalog/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type RestaurantHandler struct {
	service *service.RestaurantService
}

func NewRestaurantHandler(service *service.RestaurantService) *RestaurantHandler {
	return &RestaurantHandler{
		service: service,
	}
}

func (h *RestaurantHandler) GetRestaurants(w http.ResponseWriter, r *http.Request) {

	restaurants, err := h.service.GetRestaurants(r.Context())
	if err != nil {
		http.Error(w, "failed to get restaurants", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(restaurants); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

func (h *RestaurantHandler) GetRestaurantByID(w http.ResponseWriter, r *http.Request) {

	restaurantId := chi.URLParam(r, "restaurantId")

	parsedUUID, err := uuid.Parse(restaurantId)

	if err != nil {
		http.Error(w, "failed to parse", http.StatusBadRequest)
		return
	}

	restaurant, err := h.service.GetRestaurantByID(r.Context(), parsedUUID)

	if err != nil {
		if errors.Is(err, service.ErrRestaurantNotFound) {
			http.Error(w, "restaurant not found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to get restaurant", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(restaurant); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}
