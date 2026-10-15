// Package request parses transfer JSON.
package request

import (
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

const maxFractionDigits = 2

const maxAmountParts = 2

// TransferRequest is the POST body. Pointers keep a missing field apart from an empty string.
type TransferRequest struct {
	TransferID    *string  `json:"idempotency_key" example:"550e8400-e29b-41d4-a716-446655440000"`
	FromAccountID *string  `json:"from_account_id" example:"acc_1"`
	ToAccountID   *string  `json:"to_account_id" example:"acc_2"`
	Amount        *float64 `json:"amount" example:"100"`
}

// ParseAmount checks the JSON number text and returns a positive decimal with at most 2 fraction digits.
func ParseAmount(raw string) (decimal.Decimal, bool) {
	if raw == "" || strings.ContainsAny(raw, "eE") {
		return decimal.Decimal{}, false
	}
	body := strings.TrimPrefix(raw, "-")
	parts := strings.Split(body, ".")
	if len(parts) > maxAmountParts {
		return decimal.Decimal{}, false
	}
	if !allDigits(parts[0]) {
		return decimal.Decimal{}, false
	}
	if len(parts) == maxAmountParts && (parts[1] == "" || len(parts[1]) > maxFractionDigits || !allDigits(parts[1])) {
		return decimal.Decimal{}, false
	}
	amount, err := decimal.NewFromString(raw)
	if err != nil || !amount.IsPositive() {
		return decimal.Decimal{}, false
	}
	return amount, true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
