// Package errors is the outward error registry.
package errors

import "net/http"

// Response is the only error body.
type Response struct {
	StatusCode int    `json:"status_code" example:"422"`
	Status     string `json:"status" example:"rejected"`
	Reason     string `json:"reason" example:"insufficient funds"`
}

// Entry is one row of the registry.
type Entry struct {
	Code       string
	StatusCode int
	Status     string
	Reason     string
}

// NewError builds one registry row.
func NewError(code string, statusCode int, status, reason string) Entry {
	return Entry{
		Code:       code,
		StatusCode: statusCode,
		Status:     status,
		Reason:     reason,
	}
}

// Error returns reason.
func (e Entry) Error() string {
	return e.Reason
}

// Body copies the entry into the JSON body.
func (e Entry) Body() Response {
	return Response{
		StatusCode: e.StatusCode,
		Status:     e.Status,
		Reason:     e.Reason,
	}
}

var (
	ErrInvalidJSON                 = NewError("invalid_json", http.StatusBadRequest, "invalid", "request body is not valid json")
	ErrInvalidTransferID           = NewError("invalid_transfer_id", http.StatusBadRequest, "invalid", "idempotency_key must be a uuid")
	ErrInvalidFromAccountID        = NewError("invalid_from_account_id", http.StatusBadRequest, "invalid", "from_account_id must be non-empty")
	ErrInvalidToAccountID          = NewError("invalid_to_account_id", http.StatusBadRequest, "invalid", "to_account_id must be non-empty")
	ErrInvalidAmount               = NewError("invalid_amount", http.StatusBadRequest, "invalid", "amount must be positive with at most 2 fraction digits")
	ErrIdempotencyConflict         = NewError("idempotency_conflict", http.StatusConflict, "conflict", "idempotency key reused with different payload")
	ErrTransferTransactionConflict = NewError("transfer_transaction_conflict", http.StatusConflict, "conflict", "transfer transaction rolled back, retry the request")
	ErrTransferIDConflict          = NewError("transfer_id_conflict", http.StatusConflict, "conflict", "transfer_id already exists")
	ErrInsufficientFunds           = NewError("insufficient_funds", http.StatusUnprocessableEntity, "rejected", "insufficient funds")
	ErrSelfTransfer                = NewError("self_transfer", http.StatusUnprocessableEntity, "rejected", "self-transfer not allowed")
	ErrAccountNotFound             = NewError("account_not_found", http.StatusUnprocessableEntity, "rejected", "account not found")
	ErrAccountMissing              = NewError("account_missing", http.StatusNotFound, "not found", "account not found")
	ErrTransferMissing             = NewError("transfer_missing", http.StatusNotFound, "not found", "transfer not found")
	ErrDatabaseUnavailable         = NewError("database_unavailable", http.StatusInternalServerError, "unavailable", "postgres connection failed")
)

// Registry is the list included in the API description.
func Registry() []Entry {
	return []Entry{
		ErrInvalidJSON,
		ErrInvalidTransferID,
		ErrInvalidFromAccountID,
		ErrInvalidToAccountID,
		ErrInvalidAmount,
		ErrIdempotencyConflict,
		ErrTransferTransactionConflict,
		ErrTransferIDConflict,
		ErrInsufficientFunds,
		ErrSelfTransfer,
		ErrAccountNotFound,
		ErrAccountMissing,
		ErrTransferMissing,
		ErrDatabaseUnavailable,
	}
}
