package repository

import (
	"context"
	"errors"
	"time"

	"github.com/VitalyMars/delivery_go/services/order/internal/event"
	"github.com/VitalyMars/delivery_go/services/order/internal/servicepolicy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Order struct {
	ID              uuid.UUID
	UserID          *uuid.UUID
	RestaurantID    uuid.UUID
	Token           uuid.UUID
	CustomerName    string
	CustomerPhone   string
	DeliveryAddress string
	Comment         string
	Status          string
	Total           int
	RequestHash     string
	IdempotencyKey  string
	PaymentStatus   string
	PaymentID       *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OrderItem struct {
	ID       uuid.UUID
	OrderID  uuid.UUID
	DishID   uuid.UUID
	Name     string
	Quantity int
	Price    int
}

type OrderRepository struct {
	db               *pgxpool.Pool
	outboxRepository *OutboxRepository
}

var ErrOrderAlreadyPaid = errors.New("order already paid")

var ErrInvalidOrderToken = errors.New("invalid order token")
var ErrOrderCannotBePaid = errors.New("order cannot be paid")

var ErrOrderNotFound = errors.New("order not found")

var ErrIdempotencyKeyConflict = errors.New("idempotency key already exists")

func NewOrderRepository(db *pgxpool.Pool, outboxRepository *OutboxRepository) *OrderRepository {
	return &OrderRepository{
		db:               db,
		outboxRepository: outboxRepository,
	}
}

func (r *OrderRepository) CreateOrder(
	ctx context.Context,
	order *Order,
	items []OrderItem,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO orders (
			id,
			user_id,
			restaurant_id,
			token,
			customer_name,
			customer_phone,
			delivery_address,
			comment,
			status,
			total,
			idempotency_key, request_hash
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		`,
		order.ID,
		order.UserID,
		order.RestaurantID,
		order.Token,
		order.CustomerName,
		order.CustomerPhone,
		order.DeliveryAddress,
		order.Comment,
		order.Status,
		order.Total,
		order.IdempotencyKey, order.RequestHash,
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "idx_orders_idempotency_key" {
			return ErrIdempotencyKeyConflict
		}

		return err
	}

	for _, item := range items {
		_, err = tx.Exec(
			ctx,
			`
			INSERT INTO order_items (
				id,
				order_id,
				dish_id,
				name,
				quantity,
				price
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6
			)
			`,
			item.ID,
			item.OrderID,
			item.DishID,
			item.Name,
			item.Quantity,
			item.Price,
		)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO order_status_history (
			id,
			order_id,
			status
		)
		VALUES ($1, $2, $3)
		`,
		uuid.New(),
		order.ID,
		order.Status,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) GetOrderByIdempotencyKey(
	ctx context.Context,
	idempotencyKey string,
) (*Order, error) {
	var order Order

	err := r.db.QueryRow(
		ctx,
		`
		SELECT
			id,
			user_id,
			restaurant_id,
			token,
			customer_name,
			customer_phone,
			delivery_address,
			comment,
			status,
			total,
			idempotency_key, COALESCE(request_hash, ''),
			payment_status,
			payment_id,
			created_at,
			updated_at
		FROM orders
		WHERE idempotency_key = $1
		`,
		idempotencyKey,
	).Scan(
		&order.ID,
		&order.UserID,
		&order.RestaurantID,
		&order.Token,
		&order.CustomerName,
		&order.CustomerPhone,
		&order.DeliveryAddress,
		&order.Comment,
		&order.Status,
		&order.Total,
		&order.IdempotencyKey, &order.RequestHash,
		&order.PaymentStatus,
		&order.PaymentID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *OrderRepository) GetOrderByIDAndToken(
	ctx context.Context,
	orderID uuid.UUID,
	token uuid.UUID,
) (*Order, error) {
	var order Order

	err := r.db.QueryRow(
		ctx,
		`
		SELECT
			id,
			user_id,
			restaurant_id,
			token,
			customer_name,
			customer_phone,
			delivery_address,
			comment,
			status,
			total,
			idempotency_key, COALESCE(request_hash, ''),
    		payment_status,
    		payment_id,			
			created_at,
			updated_at
		FROM orders
		WHERE id = $1 AND token = $2
		`,
		orderID,
		token,
	).Scan(
		&order.ID,
		&order.UserID,
		&order.RestaurantID,
		&order.Token,
		&order.CustomerName,
		&order.CustomerPhone,
		&order.DeliveryAddress,
		&order.Comment,
		&order.Status,
		&order.Total,
		&order.IdempotencyKey, &order.RequestHash,
		&order.PaymentStatus,
		&order.PaymentID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *OrderRepository) GetOrderItems(
	ctx context.Context,
	orderID uuid.UUID,
) ([]OrderItem, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			id,
			order_id,
			dish_id,
			name,
			quantity,
			price
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at
		`,
		orderID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var items []OrderItem

	for rows.Next() {
		var item OrderItem

		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.DishID,
			&item.Name,
			&item.Quantity,
			&item.Price,
		)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *OrderRepository) GetOrderByID(
	ctx context.Context,
	orderID uuid.UUID,
) (*Order, error) {
	var order Order

	err := r.db.QueryRow(
		ctx,
		`
		SELECT
			id,
			user_id,
			restaurant_id,
			token,
			customer_name,
			customer_phone,
			delivery_address,
			comment,
			status,
			total,
			idempotency_key, COALESCE(request_hash, ''),
			payment_status,
			payment_id,			
			created_at,
			updated_at
		FROM orders
		WHERE id = $1
		`,
		orderID,
	).Scan(
		&order.ID,
		&order.UserID,
		&order.RestaurantID,
		&order.Token,
		&order.CustomerName,
		&order.CustomerPhone,
		&order.DeliveryAddress,
		&order.Comment,
		&order.Status,
		&order.Total,
		&order.IdempotencyKey, &order.RequestHash,
		&order.PaymentStatus,
		&order.PaymentID,
		&order.CreatedAt,
		&order.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *OrderRepository) PayForTheOrder(
	ctx context.Context,
	orderID uuid.UUID,
	token uuid.UUID,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var storedToken uuid.UUID
	var orderStatus string
	var paymentStatus string

	err = tx.QueryRow(ctx, `
		SELECT token, status, payment_status
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(
		&storedToken,
		&orderStatus,
		&paymentStatus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		return err
	}

	if storedToken != token {
		return ErrInvalidOrderToken
	}

	if paymentStatus == "PAID" {
		return ErrOrderAlreadyPaid
	}

	if orderStatus != "created" {
		return ErrOrderCannotBePaid
	}

	paymentID := uuid.New()

	_, err = tx.Exec(ctx, `
    UPDATE orders
    SET payment_status = 'PAID', status = 'ready_for_delivery',
        payment_id = $1,
        updated_at = NOW()
    WHERE id = $2
`, paymentID, orderID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, `
    SELECT dish_id, name, quantity, price
    FROM order_items
    WHERE order_id = $1
    ORDER BY id
`, orderID)
	if err != nil {
		return err
	}

	items := make([]event.OrderItem, 0)

	for rows.Next() {
		var item event.OrderItem

		if err := rows.Scan(
			&item.DishID,
			&item.Name,
			&item.Quantity,
			&item.UnitPrice,
		); err != nil {
			rows.Close()
			return err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if len(items) == 0 {
		return errors.New("order has no items")
	}

	var (
		restaurantID    uuid.UUID
		userID          *uuid.UUID
		deliveryAddress string
		total           int64
		orderCreatedAt  time.Time
	)

	err = tx.QueryRow(ctx, `
    SELECT restaurant_id, user_id, delivery_address, total, created_at
    FROM orders
    WHERE id = $1
`, orderID).Scan(
		&restaurantID,
		&userID,
		&deliveryAddress,
		&total,
		&orderCreatedAt,
	)
	if err != nil {
		return err
	}

	if restaurantID == uuid.Nil {
		return errors.New("order has no restaurant ID")
	}

	var eventUserID *string

	if userID != nil {
		value := userID.String()
		eventUserID = &value
	}

	readyEvent := event.OrderReadyForDeliveryEvent{
		EventID:         uuid.NewString(),
		UserID:          eventUserID,
		EventType:       "order.ready_for_delivery",
		OccurredAt:      time.Now(),
		OrderCreatedAt:  orderCreatedAt,
		OrderID:         orderID.String(),
		PaymentStatus:   "PAID",
		DeliveryAddress: deliveryAddress,
		Restaurants: []event.RestaurantOrder{
			{
				RestaurantID: restaurantID.String(),
				Items:        items,
			},
		},
		TotalAmount: total,
	}

	if _, err := tx.Exec(ctx, "INSERT INTO order_status_history (id,order_id,status,reason) VALUES ($1,$2,'ready_for_delivery','Demo payment confirmed')", uuid.New(), orderID); err != nil {
		return err
	}
	outboxEvent, err := createPaymentEvent(readyEvent)
	if err != nil {
		return err
	}

	if err := r.outboxRepository.CreateTx(ctx, tx, outboxEvent); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) ApplyDeliveryStatus(
	ctx context.Context,
	orderID uuid.UUID, target string,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var currentStatus string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM orders
		WHERE id = $1
		FOR UPDATE
	`, orderID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		return err
	}

	if target != "courier_assigned" && target != "delivered" {
		return errors.New("invalid delivery status")
	}
	// Different topics can arrive out of order; completion implies assignment.
	if currentStatus == target || currentStatus == "delivered" || (currentStatus == "delivering" && target == "courier_assigned") {
		return tx.Commit(ctx)
	}
	if !(target == "delivered" && currentStatus == "ready_for_delivery") && !servicepolicy.CanChangeStatus(currentStatus, target) {
		return errors.New("invalid order status transition")
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, orderID, target)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO order_status_history (id, order_id, status, reason)
		VALUES ($1, $2, $3, $4)
	`, uuid.New(), orderID, target, "Delivery event")
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
