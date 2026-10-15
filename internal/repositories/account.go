package repositories

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/shopspring/decimal"

	"github.com/ventolin13/challenge3/models"
)

const balanceScale = 2

const sqlStateClassLen = 2

var sb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// AccountRepository reads and updates account rows.
type AccountRepository interface {
	AddModel(ctx context.Context, m *models.AccountModel) error
	FindModel(ctx context.Context, filter *models.AccountFilter) ([]models.AccountModel, error)
	UpdateModel(ctx context.Context, m *models.AccountModel, partial *models.AccountPartial) (*models.AccountModel, error)
	LockBalance(ctx context.Context, accountID string) (decimal.Decimal, error)
	AddAmount(ctx context.Context, accountID string, delta decimal.Decimal) error
	EnsureSeed(ctx context.Context) error
}

// Accounts stores account rows.
type Accounts struct {
	db *DB
}

// NewAccounts returns an account repository.
func NewAccounts(db *DB) AccountRepository {
	return &Accounts{db: db}
}

// AddModel inserts an account.
func (r *Accounts) AddModel(ctx context.Context, m *models.AccountModel) error {
	query, args, err := sb.Insert("accounts").
		Columns("account_id", "balance").
		Values(m.AccountID, m.Balance.StringFixed(balanceScale)).
		ToSql()
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	_, err = r.db.conn(ctx).Exec(ctx, query, args...)
	if err != nil {
		return mapErr(err)
	}
	return nil
}

// FindModel reads accounts. It does not open a transaction and does not lock rows.
func (r *Accounts) FindModel(ctx context.Context, filter *models.AccountFilter) ([]models.AccountModel, error) {
	q := sb.Select("account_id", "balance").From("accounts")
	if filter.AccountID != nil {
		q = q.Where(sq.Eq{"account_id": *filter.AccountID})
	}
	if len(filter.AccountIDs) > 0 {
		q = q.Where(sq.Eq{"account_id": filter.AccountIDs})
	}
	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("select accounts: %w", err)
	}
	rows, err := r.db.conn(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []models.AccountModel
	for rows.Next() {
		var (
			row     models.AccountModel
			balance string
		)
		if err = rows.Scan(&row.AccountID, &balance); err != nil {
			return nil, mapErr(err)
		}
		row.Balance, err = decimal.NewFromString(balance)
		if err != nil {
			return nil, fmt.Errorf("parse balance: %w", err)
		}
		out = append(out, row)
	}
	if err = rows.Err(); err != nil {
		return nil, mapErr(err)
	}
	return out, nil
}

// UpdateModel sets balance to the absolute partial value.
func (r *Accounts) UpdateModel(ctx context.Context, m *models.AccountModel, partial *models.AccountPartial) (*models.AccountModel, error) {
	if partial.Balance == nil {
		return m, nil
	}
	query, args, err := sb.Update("accounts").
		Set("balance", partial.Balance.StringFixed(balanceScale)).
		Where(sq.Eq{"account_id": m.AccountID}).
		Suffix("RETURNING account_id, balance").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}
	var (
		id      string
		balance string
	)
	err = r.db.conn(ctx).QueryRow(ctx, query, args...).Scan(&id, &balance)
	if err != nil {
		return nil, mapErr(err)
	}
	parsed, err := decimal.NewFromString(balance)
	if err != nil {
		return nil, fmt.Errorf("parse balance: %w", err)
	}
	return &models.AccountModel{AccountID: id, Balance: parsed}, nil
}

// LockBalance reads balance of the outgoing account and locks the row.
func (r *Accounts) LockBalance(ctx context.Context, accountID string) (decimal.Decimal, error) {
	query, args, err := sb.Select("balance").
		From("accounts").
		Where(sq.Eq{"account_id": accountID}).
		Suffix("FOR UPDATE").
		ToSql()
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("lock account: %w", err)
	}
	var balance string
	err = r.db.conn(ctx).QueryRow(ctx, query, args...).Scan(&balance)
	if err != nil {
		return decimal.Decimal{}, mapErr(err)
	}
	parsed, err := decimal.NewFromString(balance)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("parse balance: %w", err)
	}
	return parsed, nil
}

// AddAmount changes balance by delta. A debit passes a negative delta.
func (r *Accounts) AddAmount(ctx context.Context, accountID string, delta decimal.Decimal) error {
	query, args, err := sb.Update("accounts").
		Set("balance", sq.Expr("balance + ?::numeric", delta.StringFixed(balanceScale))).
		Where(sq.Eq{"account_id": accountID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("add amount: %w", err)
	}
	tag, err := r.db.conn(ctx).Exec(ctx, query, args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w", ErrNotFound)
	}
	return nil
}

// EnsureSeed inserts acc_1..acc_10 with balance 1000 when the row is absent.
func (r *Accounts) EnsureSeed(ctx context.Context) error {
	const (
		seedCount   = 10
		seedBalance = 1000
	)
	for n := 1; n <= seedCount; n++ {
		query, args, err := sb.Insert("accounts").
			Columns("account_id", "balance").
			Values(fmt.Sprintf("acc_%d", n), decimal.NewFromInt(seedBalance).StringFixed(balanceScale)).
			Suffix("ON CONFLICT (account_id) DO NOTHING").
			ToSql()
		if err != nil {
			return fmt.Errorf("seed account: %w", err)
		}
		if _, err = r.db.conn(ctx).Exec(ctx, query, args...); err != nil {
			return mapErr(err)
		}
	}
	return nil
}
