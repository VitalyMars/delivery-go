package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/VitalyMars/delivery_go/services/order/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CartHandler struct {
	service *service.CartService
}

func NewCartHandler(service *service.CartService) *CartHandler {
	return &CartHandler{
		service: service,
	}
}

type AddCartItemRequest struct {
	CartToken string `json:"cart_token"`
	DishID    string `json:"dish_id"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

type AddCartItemResponse struct {
	ID        string `json:"id"`
	CartToken string `json:"cart_token"`
	DishID    string `json:"dish_id"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

func (h *CartHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	var req AddCartItemRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cartToken, err := uuid.Parse(req.CartToken)
	if err != nil {
		http.Error(w, "invalid cart token", http.StatusBadRequest)
		return
	}

	dishID, err := uuid.Parse(req.DishID)
	if err != nil {
		http.Error(w, "invalid dish id", http.StatusBadRequest)
		return
	}

	item, err := h.service.AddItem(
		r.Context(),
		service.AddCartItemInput{
			CartToken: cartToken,
			DishID:    dishID,
			Quantity:  req.Quantity,
			Price:     req.Price,
		},
	)

	if err != nil {
		http.Error(w, "failed to add cart item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(AddCartItemResponse{
		ID:        item.ID.String(),
		CartToken: item.CartToken.String(),
		DishID:    item.DishID.String(),
		Quantity:  item.Quantity,
		Price:     item.Price,
	}); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

type GetCartResponse struct {
	Items []CartItemResponse
}

type CartItemResponse struct {
	ID        string `json:"id"`
	CartToken string `json:"cart_token"`
	DishID    string `json:"dish_id"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

func (h *CartHandler) GetCart(w http.ResponseWriter, r *http.Request) {
	cartToken, err := uuid.Parse(r.URL.Query().Get("cart_token"))

	if err != nil {
		http.Error(w, "invalid cart token", http.StatusBadRequest)
		return
	}

	items, err := h.service.GetItems(
		r.Context(),
		cartToken,
	)

	if err != nil {
		http.Error(w, "failed to get cart", http.StatusInternalServerError)
		return
	}

	response := GetCartResponse{
		Items: make([]CartItemResponse, 0, len(items)),
	}

	for _, item := range items {
		response.Items = append(response.Items, CartItemResponse{
			ID:        item.ID.String(),
			CartToken: item.CartToken.String(),
			DishID:    item.DishID.String(),
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

type UpdateCartitemRequest struct {
	CartToken string `json:"cart_token"`
	Quantity  int    `json:"quantity"`
}

func (h *CartHandler) UpdateItemQuantity(w http.ResponseWriter, r *http.Request) {

	itemID, err := uuid.Parse(chi.URLParam(r, "itemId"))
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	var req UpdateCartitemRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cartToken, err := uuid.Parse(req.CartToken)
	if err != nil {
		http.Error(w, "invalid cart token", http.StatusBadRequest)
		return
	}

	if req.Quantity <= 0 {
		http.Error(w, "invalid quantity", http.StatusBadRequest)
		return
	}

	items, found, err := h.service.UpdateItemQuantity(
		r.Context(),
		cartToken,
		itemID,
		req.Quantity,
	)

	if err != nil {
		log.Printf("failed to update cart item: %v", err)
		http.Error(w, "failed to update cart item", http.StatusInternalServerError)
		return
	}

	if !found {
		http.Error(w, "cart item not found", http.StatusNotFound)
		return
	}

	response := GetCartResponse{
		Items: make([]CartItemResponse, 0, len(items)),
	}

	for _, item := range items {
		response.Items = append(response.Items, CartItemResponse{
			ID:        item.ID.String(),
			CartToken: item.CartToken.String(),
			DishID:    item.DishID.String(),
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}

}

type DeleteCartItemRequest struct {
	CartToken string `json:"cart_token"`
}

func (h *CartHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {

	itemID, err := uuid.Parse(chi.URLParam(r, "itemId"))
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	var req DeleteCartItemRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cartToken, err := uuid.Parse(req.CartToken)
	if err != nil {
		http.Error(w, "invalid cart token", http.StatusBadRequest)
		return
	}

	items, found, err := h.service.DeleteItem(
		r.Context(),
		cartToken,
		itemID,
	)

	if err != nil {
		log.Printf("failed to delete cart item: %v", err)
		http.Error(w, "failed to delete cart item", http.StatusInternalServerError)
		return
	}

	if !found {
		http.Error(w, "cart item not found", http.StatusNotFound)
		return
	}

	response := GetCartResponse{
		Items: make([]CartItemResponse, 0, len(items)),
	}

	for _, item := range items {
		response.Items = append(response.Items, CartItemResponse{
			ID:        item.ID.String(),
			CartToken: item.CartToken.String(),
			DishID:    item.DishID.String(),
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
