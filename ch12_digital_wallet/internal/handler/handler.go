package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nathanyu/digital-wallet/internal/cqrs"
	"github.com/nathanyu/digital-wallet/internal/domain"
	"github.com/nathanyu/digital-wallet/internal/engine"
	"github.com/nathanyu/digital-wallet/internal/queue"
)

// Handler contains all HTTP handlers
type Handler struct {
	natsClient   *queue.NATSClient
	readModel    *cqrs.ReadModel
	walletEngine *engine.WalletEngine
	timeout      time.Duration
}

// NewHandler creates a new handler
func NewHandler(natsClient *queue.NATSClient, readModel *cqrs.ReadModel, walletEngine *engine.WalletEngine) *Handler {
	return &Handler{
		natsClient:   natsClient,
		readModel:    readModel,
		walletEngine: walletEngine,
		timeout:      5 * time.Second,
	}
}

// TransferRequest is the request body for transfer endpoint
type TransferRequest struct {
	FromAccount   string `json:"from_account" binding:"required"`
	ToAccount     string `json:"to_account" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
	TransactionID string `json:"transaction_id"` // Optional, will be generated if not provided
}

// TransferResponse is the response body for transfer endpoint
type TransferResponse struct {
	TransactionID string   `json:"transaction_id"`
	Success       bool     `json:"success"`
	Message       string   `json:"message,omitempty"`
	Events        []string `json:"events,omitempty"`
}

// Transfer handles POST /v1/wallet/transfer
func (h *Handler) Transfer(c *gin.Context) {
	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Generate transaction ID if not provided
	txnID := req.TransactionID
	if txnID == "" {
		txnID = uuid.Must(uuid.NewV7()).String()
	}

	// Create command
	cmd := domain.TransferCommand{
		TransactionID: txnID,
		FromAccount:   req.FromAccount,
		ToAccount:     req.ToAccount,
		Amount:        req.Amount,
	}

	// Publish command and wait for response
	resp, err := h.natsClient.PublishCommand(cmd, h.timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":          "failed to process transfer",
			"transaction_id": txnID,
		})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusBadRequest, TransferResponse{
			TransactionID: txnID,
			Success:       false,
			Message:       resp.Error,
		})
		return
	}

	c.JSON(http.StatusOK, TransferResponse{
		TransactionID: txnID,
		Success:       true,
		Message:       "transfer completed",
		Events:        resp.Events,
	})
}

// BalanceResponse is the response body for balance endpoint
type BalanceResponse struct {
	Account string `json:"account"`
	Balance int64  `json:"balance"`
}

// GetBalance handles GET /v1/wallet/balance/:account_id
func (h *Handler) GetBalance(c *gin.Context) {
	accountID := c.Param("account_id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "account_id is required",
		})
		return
	}

	balance, exists := h.readModel.GetBalance(accountID)
	if !exists {
		// Return 0 balance for non-existent accounts
		c.JSON(http.StatusOK, BalanceResponse{
			Account: accountID,
			Balance: 0,
		})
		return
	}

	c.JSON(http.StatusOK, BalanceResponse{
		Account: accountID,
		Balance: balance,
	})
}

// AllBalancesResponse is the response for all balances endpoint
type AllBalancesResponse struct {
	Balances     map[string]int64 `json:"balances"`
	TotalBalance int64            `json:"total_balance"`
	AccountCount int              `json:"account_count"`
}

// GetAllBalances handles GET /v1/wallet/balances
func (h *Handler) GetAllBalances(c *gin.Context) {
	balances := h.readModel.GetAllBalances()
	total := h.readModel.GetTotalBalance()

	c.JSON(http.StatusOK, AllBalancesResponse{
		Balances:     balances,
		TotalBalance: total,
		AccountCount: len(balances),
	})
}

// HealthResponse is the response for health check endpoint
type HealthResponse struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

// Health handles GET /health
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status: "ok",
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
}

// InitAccountRequest is the request body for account initialization
type InitAccountRequest struct {
	Account string `json:"account" binding:"required"`
	Balance int64  `json:"balance" binding:"required,gte=0"`
}

// InitAccount handles POST /v1/wallet/init (for testing purposes)
func (h *Handler) InitAccount(c *gin.Context) {
	var req InitAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Update both the wallet engine (for validation) and read model (for queries)
	h.walletEngine.SetBalance(req.Account, req.Balance)
	h.readModel.SetBalance(req.Account, req.Balance)

	c.JSON(http.StatusOK, gin.H{
		"message": "account initialized",
		"account": req.Account,
		"balance": req.Balance,
	})
}

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine, h *Handler) {
	// Health check
	r.GET("/health", h.Health)

	// API v1
	v1 := r.Group("/v1/wallet")
	{
		v1.POST("/transfer", h.Transfer)
		v1.GET("/balance/:account_id", h.GetBalance)
		v1.GET("/balances", h.GetAllBalances)
		v1.POST("/init", h.InitAccount) // For testing
	}
}
