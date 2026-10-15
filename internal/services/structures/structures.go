// Package structures holds business values.
package structures

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Transfer is a decided transfer. An empty Reason means there is no rejection.
type Transfer struct {
	TransferID    uuid.UUID
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
	Status        string
	Reason        string
	CreatedAt     time.Time
}

// Account is an account balance.
type Account struct {
	AccountID string
	Balance   decimal.Decimal
}

// TransferCommand is a validated transfer request.
type TransferCommand struct {
	TransferID    uuid.UUID
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
}
