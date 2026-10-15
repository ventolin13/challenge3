package repositories

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/shopspring/decimal"

	"github.com/ventolin13/challenge3/models"
)

// TransferRepository reads and inserts transfer rows.
type TransferRepository interface {
	AddModel(ctx context.Context, m *models.TransferModel) error
	FindModel(ctx context.Context, filter *models.TransferFilter) ([]models.TransferModel, error)
}

// Transfers stores transfer rows.
type Transfers struct {
	db *DB
}

// NewTransfers returns a transfer repository.
func NewTransfers(db *DB) TransferRepository {
	return &Transfers{db: db}
}

// AddModel inserts a completed movement or an insufficient funds rejection.
func (r *Transfers) AddModel(ctx context.Context, m *models.TransferModel) error {
	var reason any
	if m.Reason.Valid {
		reason = m.Reason.String
	}
	query, args, err := sb.Insert("transfers").
		Columns("transfer_id", "from_account_id", "to_account_id", "amount", "status", "reason", "created_at").
		Values(m.TransferID, m.FromAccountID, m.ToAccountID, m.Amount.StringFixed(balanceScale), m.Status, reason, m.CreatedAt.UTC()).
		ToSql()
	if err != nil {
		return fmt.Errorf("insert transfer: %w", err)
	}
	_, err = r.db.conn(ctx).Exec(ctx, query, args...)
	if err != nil {
		return mapErr(err)
	}
	return nil
}

// FindModel reads transfers. It does not open a transaction and does not lock rows.
func (r *Transfers) FindModel(ctx context.Context, filter *models.TransferFilter) ([]models.TransferModel, error) {
	q := sb.Select("transfer_id", "from_account_id", "to_account_id", "amount", "status", "reason", "created_at").
		From("transfers")
	if filter.TransferID != nil {
		q = q.Where(sq.Eq{"transfer_id": *filter.TransferID})
	}
	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("select transfers: %w", err)
	}
	rows, err := r.db.conn(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []models.TransferModel
	for rows.Next() {
		var (
			row    models.TransferModel
			amount string
		)
		if err = rows.Scan(
			&row.TransferID,
			&row.FromAccountID,
			&row.ToAccountID,
			&amount,
			&row.Status,
			&row.Reason,
			&row.CreatedAt,
		); err != nil {
			return nil, mapErr(err)
		}
		row.Amount, err = decimal.NewFromString(amount)
		if err != nil {
			return nil, fmt.Errorf("parse amount: %w", err)
		}
		row.CreatedAt = row.CreatedAt.UTC()
		out = append(out, row)
	}
	if err = rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	return out, nil
}
