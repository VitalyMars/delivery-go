package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VitalyMars/delivery_go/services/catalog/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DishHandler struct {
	service *service.DishService
}

func NewDishHandler(service *service.DishService) *DishHandler {
	return &DishHandler{
		service: service,
	}
}

func (h *DishHandler) GetDishesByRestaurantID(w http.ResponseWriter, r *http.Request) {
	restaurantId := chi.URLParam(r, "restaurantId")

	parsedUUID, err := uuid.Parse(restaurantId)

	if err != nil {
		http.Error(w, "failed to parse", http.StatusBadRequest)
		return
	}

	dishes, err := h.service.GetDishesByRestaurantID(r.Context(), parsedUUID)

	if err != nil {
		http.Error(w, "failed to get dishes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(dishes); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

func (h *DishHandler) GetDishByID(w http.ResponseWriter, r *http.Request) {
	dishId := chi.URLParam(r, "dishId")

	parsedUUID, err := uuid.Parse(dishId)

	if err != nil {
		http.Error(w, "failed to parse", http.StatusBadRequest)
		return
	}

	dish, err := h.service.GetDishByID(r.Context(), parsedUUID)
	if err != nil {
		if errors.Is(err, service.ErrDishNotFound) {
			http.Error(w, "dish not found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to get dish", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(dish); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}
