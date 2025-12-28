package domain

// TransferCommand represents a transfer request from the API
type TransferCommand struct {
	TransactionID string `json:"transaction_id"`
	FromAccount   string `json:"from_account"`
	ToAccount     string `json:"to_account"`
	Amount        int64  `json:"amount"` // Amount in cents to avoid floating point issues
}
