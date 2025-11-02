package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"example.com/v2/internal/api"
	"example.com/v2/internal/event"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/rs/zerolog"
)

// Server is your implementation of ServerInterface
type Server struct {
	db         *sql.DB
	eventQueue chan event.OrderCreated
	logger     zerolog.Logger
}

func NewServer(db *sql.DB, eventQueue chan event.OrderCreated, logger zerolog.Logger) *Server {
	return &Server{
		db:         db,
		eventQueue: eventQueue,
		logger:     logger,
	}
}

// HealthCheck handles GET /healthz
func (s *Server) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CreateOrder handles POST /orders
func (s *Server) CreateOrder(c *gin.Context) {
	logger := s.logger.With().Str("handler", "CreateOrder").Logger()

	var req api.CreateOrderJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).Msg("Invalid input")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	createdAt := time.Now().UTC()
	var existingOrder api.OrderResponse

	query := `SELECT id, customer_name, item, created_at FROM orders 
              WHERE customer_name=$1 AND item=$2 AND created_at=$3 LIMIT 1`
	row := s.db.QueryRowContext(c.Request.Context(), query, req.CustomerName, req.Item, createdAt)
	err := row.Scan(&existingOrder.Id, &existingOrder.CustomerName, &existingOrder.Item, &existingOrder.CreatedAt)
	if err == nil {
		// Found existing order: return it with 200 OK
		logger.Info().
			Str("customer_name", req.CustomerName).
			Str("item", req.Item).
			Time("created_at", createdAt).
			Msg("Duplicate order detected - returning existing order")
		c.JSON(http.StatusOK, existingOrder)
		return
	} else if err != sql.ErrNoRows {
		// Database error - log and return error
		logger.Error().Err(err).Msg("DB error checking existing order")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	id := openapi_types.UUID(uuid.New())
	// Insert into DB
	query = `INSERT INTO orders (id, customer_name, item, created_at) VALUES ($1, $2, $3, $4)`
	if _, err := s.db.ExecContext(c.Request.Context(), query, id, req.CustomerName, req.Item, createdAt); err != nil {
		logger.Error().Err(err).Msg("DB insert failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err,
			"msg":   "Failed to create order",
		})
		return
	}

	logger.Info().
		Str("order_id", id.String()).
		Str("customer", req.CustomerName).
		Msg("Order created")

	// Return created order response
	resp := api.OrderResponse{
		Id:           &id,
		CustomerName: &req.CustomerName,
		Item:         &req.Item,
		CreatedAt:    &createdAt,
	}
	go func() {
		s.eventQueue <- event.OrderCreated{Order: resp}
	}()
	c.JSON(http.StatusCreated, resp)
}

// GetOrderById handles GET /orders/{id}
func (s *Server) GetOrderById(c *gin.Context, id openapi_types.UUID) {
	logger := s.logger.With().Str("handler", "GetOrderById").Logger()

	var order api.OrderResponse
	query := `SELECT id, customer_name, item, created_at FROM orders WHERE id = $1`
	row := s.db.QueryRowContext(c.Request.Context(), query, id)
	err := row.Scan(&order.Id, &order.CustomerName, &order.Item, &order.CreatedAt)
	if err == sql.ErrNoRows {
		logger.Error().Err(err).Msg("Order not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	} else if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch order")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch order"})
		return
	}

	logger.Info().
		Str("order_id", id.String()).
		Str("order", fmt.Sprintf("%+v", order)).
		Msg("Order found")

	c.JSON(http.StatusOK, order)
}
