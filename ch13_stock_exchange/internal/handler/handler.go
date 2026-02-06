package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nathanyu/stock-exchange/internal/domain"
	"github.com/nathanyu/stock-exchange/internal/matching"
	"github.com/nathanyu/stock-exchange/internal/marketdata"
	"github.com/nathanyu/stock-exchange/internal/ordermanager"
)

// Handler holds the HTTP handler dependencies.
type Handler struct {
	manager   *ordermanager.Manager
	engine    *matching.Engine
	publisher *marketdata.Publisher
}

// NewHandler creates a new Handler.
func NewHandler(manager *ordermanager.Manager, engine *matching.Engine, publisher *marketdata.Publisher) *Handler {
	return &Handler{
		manager:   manager,
		engine:    engine,
		publisher: publisher,
	}
}

// RegisterRoutes sets up the Gin routes.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.Health)

	v1 := r.Group("/v1")
	{
		v1.POST("/order", h.PlaceOrder)
		v1.DELETE("/order/:id", h.CancelOrder)
		v1.GET("/execution", h.GetExecutions)
		v1.GET("/marketdata/orderBook/L2", h.GetL2OrderBook)
		v1.GET("/marketdata/candles", h.GetCandles)
		v1.GET("/wallet/balances", h.GetBalances)
		v1.POST("/wallet/init", h.InitWallet)
	}
}

// Health returns a health check response.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "stock-exchange",
	})
}

// PlaceOrderRequest is the request body for placing an order.
type PlaceOrderRequest struct {
	Symbol   string      `json:"symbol" binding:"required"`
	Side     domain.Side `json:"side" binding:"required"`
	Price    int64       `json:"price" binding:"required,gt=0"`
	Quantity int64       `json:"quantity" binding:"required,gt=0"`
	UserID   string      `json:"user_id" binding:"required"`
}

// PlaceOrder handles POST /v1/order.
func (h *Handler) PlaceOrder(c *gin.Context) {
	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Side != domain.SideBuy && req.Side != domain.SideSell {
		c.JSON(http.StatusBadRequest, gin.H{"error": "side must be 'buy' or 'sell'"})
		return
	}

	order, err := h.manager.PlaceOrder(req.UserID, req.Symbol, req.Side, req.Price, req.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// CancelOrder handles DELETE /v1/order/:id.
func (h *Handler) CancelOrder(c *gin.Context) {
	orderID := c.Param("id")

	order, err := h.manager.CancelOrder(orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetExecutions handles GET /v1/execution.
func (h *Handler) GetExecutions(c *gin.Context) {
	symbol := c.Query("symbol")
	orderID := c.Query("order_id")
	sinceStr := c.Query("since")

	var since time.Time
	if sinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since format, use RFC3339"})
			return
		}
		since = parsed
	}

	executions := h.publisher.GetExecutions(symbol, orderID, since)
	if executions == nil {
		executions = []*domain.Execution{}
	}

	c.JSON(http.StatusOK, executions)
}

// GetL2OrderBook handles GET /v1/marketdata/orderBook/L2.
func (h *Handler) GetL2OrderBook(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	depthStr := c.DefaultQuery("depth", "10")
	depth, err := strconv.Atoi(depthStr)
	if err != nil || depth <= 0 {
		depth = 10
	}

	snapshot := h.engine.GetL2Snapshot(symbol, depth)
	c.JSON(http.StatusOK, snapshot)
}

// GetCandles handles GET /v1/marketdata/candles.
func (h *Handler) GetCandles(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	countStr := c.DefaultQuery("count", "100")
	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 {
		count = 100
	}

	candles := h.publisher.GetCandles(symbol, count)
	if candles == nil {
		candles = []*domain.Candlestick{}
	}

	c.JSON(http.StatusOK, candles)
}

// InitWalletRequest is the request body for initializing a wallet.
type InitWalletRequest struct {
	UserID      string           `json:"user_id" binding:"required"`
	CashBalance int64            `json:"cash_balance" binding:"required"`
	Holdings    map[string]int64 `json:"holdings"`
}

// InitWallet handles POST /v1/wallet/init.
func (h *Handler) InitWallet(c *gin.Context) {
	var req InitWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Holdings == nil {
		req.Holdings = make(map[string]int64)
	}

	h.manager.InitWallet(req.UserID, req.CashBalance, req.Holdings)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"user_id": req.UserID,
	})
}

// GetBalances handles GET /v1/wallet/balances.
func (h *Handler) GetBalances(c *gin.Context) {
	userID := c.Query("user_id")
	if userID != "" {
		wallet := h.manager.GetWallet(userID)
		if wallet == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user_id":      userID,
			"cash_balance": wallet.CashBalance,
			"holdings":     wallet.Holdings,
		})
		return
	}

	wallets := h.manager.GetAllWallets()
	result := make([]gin.H, 0, len(wallets))
	for uid, w := range wallets {
		result = append(result, gin.H{
			"user_id":      uid,
			"cash_balance": w.CashBalance,
			"holdings":     w.Holdings,
		})
	}
	c.JSON(http.StatusOK, result)
}
