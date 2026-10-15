// Package models holds repository rows.
package models

import "github.com/shopspring/decimal"

// AccountModel is one account row.
type AccountModel struct {
	AccountID string
	Balance   decimal.Decimal
}

// AccountFilter selects accounts. A nil criterion is not applied.
type AccountFilter struct {
	AccountID  *string
	AccountIDs []string
}

// AccountPartial lists balance fields present on update.
type AccountPartial struct {
	Balance *decimal.Decimal
}
