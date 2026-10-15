// Package response is the HTTP view of a transfer and an account.
package response

import (
	"time"

	"github.com/ventolin13/challenge3/internal/services/structures"
)

// TransferResponse is the body of a completed post and of a transfer read.
// reason is present only for a stored rejection. The other fields are always set.
type TransferResponse struct {
	TransferID     string  `json:"transfer_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	IdempotencyKey string  `json:"idempotency_key" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status         string  `json:"status" binding:"required" enums:"completed,rejected" example:"completed"`
	StatusCode     int     `json:"status_code" binding:"required" example:"200"`
	FromAccountID  string  `json:"from_account_id" binding:"required" example:"acc_1"`
	ToAccountID    string  `json:"to_account_id" binding:"required" example:"acc_2"`
	Amount         float64 `json:"amount" binding:"required" example:"100"`
	Reason         *string `json:"reason,omitempty" example:"insufficient funds"`
	CreatedAt      *string `json:"created_at" binding:"required" example:"2025-01-01T00:00:00Z"`
}

// AccountResponse is the balance body. Both fields are always set.
type AccountResponse struct {
	AccountID string  `json:"account_id" binding:"required" example:"acc_1"`
	Balance   float64 `json:"balance" binding:"required" example:"900"`
}

// NewTransfer builds the JSON view. created_at is set from the stored row.
func NewTransfer(row *structures.Transfer, statusCode int) TransferResponse {
	created := row.CreatedAt.UTC().Format(time.RFC3339)
	view := TransferResponse{
		TransferID:     row.TransferID.String(),
		IdempotencyKey: row.TransferID.String(),
		Status:         row.Status,
		StatusCode:     statusCode,
		FromAccountID:  row.FromAccountID,
		ToAccountID:    row.ToAccountID,
		Amount:         row.Amount.InexactFloat64(),
		Reason:         nil,
		CreatedAt:      &created,
	}
	if row.Reason != "" {
		reason := row.Reason
		view.Reason = &reason
	}
	return view
}

// NewAccount builds the balance JSON view.
func NewAccount(row structures.Account) AccountResponse {
	return AccountResponse{
		AccountID: row.AccountID,
		Balance:   row.Balance.InexactFloat64(),
	}
}
