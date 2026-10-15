package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// StatusCompleted is a stored movement.
const StatusCompleted = "completed"

// StatusRejected is a stored insufficient funds outcome.
const StatusRejected = "rejected"

// TransferModel is a stored transfer outcome.
type TransferModel struct {
	TransferID    uuid.UUID
	FromAccountID string
	ToAccountID   string
	Amount        decimal.Decimal
	Status        string
	Reason        sql.NullString
	CreatedAt     time.Time
}

// TransferFilter selects transfers by transfer_id.
type TransferFilter struct {
	TransferID *uuid.UUID
}

// TransferPartial is empty: a stored row is not rewritten.
type TransferPartial struct{}
