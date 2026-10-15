package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	catalogv1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/VitalyMars/delivery_go/services/order/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type OrderHandler struct {
	service *service.OrderService
}

func NewOrderHandler(service *service.OrderService) *OrderHandler {
	return &OrderHandler{
		service: service,
	}
}

type CreateOrderResponse struct {
	OrderID    string `json:"order_id"`
	OrderToken string `json:"order_token"`
}

type GetOrderResponse struct {
	OrderID         string              `json:"order_id"`
	OrderToken      string              `json:"order_token"`
	CustomerName    string              `json:"customer_name"`
	CustomerPhone   string              `json:"customer_phone"`
	DeliveryAddress string              `json:"delivery_address"`
	Comment         string              `json:"comment"`
	Status          string              `json:"status"`
	Total           int                 `json:"total"`
	PaymentStatus   string              `json:"payment_status"`
	PaymentID       *uuid.UUID          `json:"payment_id,omitempty"`
	Items           []OrderItemResponse `json:"items"`
}

type OrderItemResponse struct {
	DishID   string `json:"dish_id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

type CreateOrderRequest struct {
	UserID          *uuid.UUID               `json:"user_id,omitempty"`
	CustomerName    string                   `json:"customer_name"`
	CustomerPhone   string                   `json:"customer_phone"`
	DeliveryAddress string                   `json:"delivery_address"`
	Comment         string                   `json:"comment"`
	IdempotencyKey  string                   `json:"idempotency_key"`
	RestaurantID    string                   `json:"restaurant_id"`
	Items           []CreateOrderItemRequest `json:"items"`
}

type CreateOrderItemRequest struct {
	DishID   string `json:"dish_id"`
	Quantity int32  `json:"quantity"`
}

type PayForTheOrderResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("decoded request: %+v", req)

	items := make([]*catalogv1.OrderItem, 0, len(req.Items))

	for _, item := range req.Items {
		items = append(items, &catalogv1.OrderItem{
			DishId:   item.DishID,
			Quantity: item.Quantity,
		})
	}

	for _, item := range items {
		log.Printf(
			"handler item: dish_id=%q quantity=%d",
			item.GetDishId(),
			item.GetQuantity(),
		)
	}

	order, err := h.service.CreateOrder(
		r.Context(),
		service.CreateOrderInput{
			UserID:          req.UserID,
			CustomerName:    req.CustomerName,
			CustomerPhone:   req.CustomerPhone,
			DeliveryAddress: req.DeliveryAddress,
			Comment:         req.Comment,
			IdempotencyKey:  req.IdempotencyKey,
			RestaurantID:    req.RestaurantID,
			Items:           items,
		},
	)

	if err != nil {
		log.Printf("create order error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(CreateOrderResponse{
		OrderID:    order.ID.String(),
		OrderToken: order.Token.String(),
	}); err != nil {
		http.Error(w, "failed to encode", http.StatusInternalServerError)
		return
	}
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	token, err := uuid.Parse(r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}

	result, err := h.service.GetOrder(
		r.Context(),
		orderID,
		token,
	)
	if err != nil {
		log.Printf("get order error: %v", err)
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	items := make([]OrderItemResponse, 0, len(result.Items))

	for _, item := range result.Items {
		items = append(items, OrderItemResponse{
			DishID:   item.DishID.String(),
			Name:     item.Name,
			Quantity: item.Quantity,
			Price:    item.Price,
		})
	}

	response := GetOrderResponse{
		OrderID:         result.Order.ID.String(),
		OrderToken:      result.Order.Token.String(),
		CustomerName:    result.Order.CustomerName,
		CustomerPhone:   result.Order.CustomerPhone,
		DeliveryAddress: result.Order.DeliveryAddress,
		Comment:         result.Order.Comment,
		Status:          result.Order.Status,
		Total:           result.Order.Total,
		PaymentStatus:   result.Order.PaymentStatus,
		PaymentID:       result.Order.PaymentID,
		Items:           items,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *OrderHandler) PayForTheOrder(w http.ResponseWriter, r *http.Request) {

	orderID, err := uuid.Parse(chi.URLParam(r, "orderId"))
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	token, err := uuid.Parse(r.URL.Query().Get("token"))
	if err != nil {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}

	if err := h.service.PayForTheOrder(
		r.Context(),
		orderID,
		token,
	); err != nil {
		switch {
		case errors.Is(err, repository.ErrOrderNotFound):
			http.Error(w, "order not found", http.StatusNotFound)

		case errors.Is(err, service.ErrInvalidOrderToken):
			http.Error(w, "invalid order token", http.StatusForbidden)

		case errors.Is(err, repository.ErrOrderAlreadyPaid):
			response := PayForTheOrderResponse{
				Success: true,
				Message: "Your order has already been paid",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
				return
			}

		case errors.Is(err, service.ErrOrderCannotBePaid):
			http.Error(w, "order cannot be paid", http.StatusConflict)

		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	response := PayForTheOrderResponse{
		Success: true,
		Message: "Your order has been successfully paid for",
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
