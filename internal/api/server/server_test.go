package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/v2/internal/api"
	"example.com/v2/internal/event"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func setupServerWithMockDB(t *testing.T) (*Server, sqlmock.Sqlmock, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	logger := zerolog.Nop()
	ch := make(chan event.OrderCreated, 100)
	eventWorker := event.NewEventWorker(ch, logger)
	eventWorker.StartEventWorker(ctx)
	srv := NewServer(db, ch, logger)
	return srv, mock, cancel
}

func setupGinTestContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestHealthCheck(t *testing.T) {
	srv := Server{}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/healthz", srv.HealthCheck)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestCreateOrder_Success(t *testing.T) {
	srv, mock, cancel := setupServerWithMockDB(t)
	defer cancel()
	reqBody := `{"customer_name": "John Doe", "item": "Book"}`

	c, w := setupGinTestContext(reqBody)

	// Expect query for existing order by customerName, item, createdAt (example)
	rows := sqlmock.NewRows([]string{"id", "customer_name", "item", "created_at"})

	mock.ExpectQuery(`SELECT .* FROM orders WHERE customer_name.*`).
		WithArgs("John Doe", "Book", sqlmock.AnyArg()).
		WillReturnRows(rows)

	// Mock DB Exec for order insertion
	mock.ExpectExec("INSERT INTO orders").
		WithArgs(sqlmock.AnyArg(), "John Doe", "Book", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	srv.CreateOrder(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp api.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.NotNil(t, resp.CustomerName)
	assert.Equal(t, "John Doe", *resp.CustomerName)

	assert.NotNil(t, resp.Item)
	assert.Equal(t, "Book", *resp.Item)

	assert.NotEmpty(t, resp.Id.String())
	assert.False(t, resp.CreatedAt.IsZero())
}

func TestCreateOrder_InvalidJSON(t *testing.T) {
	srv, _, cancel := setupServerWithMockDB(t)
	defer cancel()
	reqBody := `{"customer_name": 12345, "item": []}` // invalid types

	c, w := setupGinTestContext(reqBody)

	srv.CreateOrder(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrder_DBError(t *testing.T) {
	srv, mock, cancel := setupServerWithMockDB(t)
	defer cancel()
	reqBody := `{"customer_name": "John Doe", "item": "Book"}`

	c, w := setupGinTestContext(reqBody)

	mock.ExpectExec("INSERT INTO orders").
		WithArgs(sqlmock.AnyArg(), "John Doe", "Book", sqlmock.AnyArg()).
		WillReturnError(errors.New("db error"))

	srv.CreateOrder(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateOrder_IdempotencyFound(t *testing.T) {
	srv, mock, cancel := setupServerWithMockDB(t)
	defer cancel()
	reqBody := `{"customer_name": "John Doe", "item": "Book"}`

	c, w := setupGinTestContext(reqBody)

	orderID := uuid.New()
	createdAt := time.Now()

	// Expect query for existing order by customerName, item, createdAt (example)
	rows := sqlmock.NewRows([]string{"id", "customer_name", "item", "created_at"}).
		AddRow(orderID, "John Doe", "Book", createdAt)

	mock.ExpectQuery(`SELECT .* FROM orders WHERE customer_name.*`).
		WithArgs("John Doe", "Book", sqlmock.AnyArg()).
		WillReturnRows(rows)

	srv.CreateOrder(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.OrderResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.NotNil(t, resp.CustomerName)
	assert.Equal(t, "John Doe", *resp.CustomerName)
	assert.Equal(t, "Book", *resp.Item)
	assert.Equal(t, orderID.String(), resp.Id.String())
}

func TestGetOrderById_Success(t *testing.T) {
	srv, mock, cancel := setupServerWithMockDB(t)
	defer cancel()

	orderID := uuid.New()
	rows := sqlmock.NewRows([]string{"id", "customer_name", "item", "created_at"}).
		AddRow(orderID, "John Doe", "Book", time.Now())

	mock.ExpectQuery("SELECT id, customer_name, item, created_at FROM orders WHERE id =").
		WithArgs(orderID).
		WillReturnRows(rows)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/orders/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		srv.GetOrderById(c, id)
	}) // adapt if path differs

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
func TestGetOrderById_NotFound(t *testing.T) {
	srv, mock, cancel := setupServerWithMockDB(t)
	defer cancel()
	orderID := uuid.New()

	mock.ExpectQuery("SELECT id, customer_name, item, created_at FROM orders WHERE id =").
		WithArgs(orderID).
		WillReturnError(sql.ErrNoRows)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/orders/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		srv.GetOrderById(c, id)
	}) // adapt if path differs

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetOrderById_DBError(t *testing.T) {
	srv, mock, cancel := setupServerWithMockDB(t)
	defer cancel()
	orderID := uuid.New()

	mock.ExpectQuery("SELECT id, customer_name, item, created_at FROM orders WHERE id =").
		WithArgs(orderID).
		WillReturnError(errors.New("db error"))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/orders/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		srv.GetOrderById(c, id)
	}) // adapt if path differs

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/orders/"+orderID.String(), nil)

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
