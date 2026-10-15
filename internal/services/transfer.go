// Package services decides transfers.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/ventolin13/challenge3/errors"
	"github.com/ventolin13/challenge3/internal/repositories"
	"github.com/ventolin13/challenge3/internal/services/structures"
	"github.com/ventolin13/challenge3/models"
)

// ErrIdempotencyConflict is the same transfer_id with another body.
var ErrIdempotencyConflict = errors.New("idempotency key reused with different payload")

// ErrTransactionConflict is a rolled back transfer transaction.
var ErrTransactionConflict = errors.New("transfer transaction rolled back, retry the request")

// ErrTransferIDExists is a taken transfer_id.
var ErrTransferIDExists = errors.New("transfer_id already exists")

// ErrInsufficientFunds is a rejected transfer.
var ErrInsufficientFunds = errors.New("insufficient funds")

// ErrSelfTransfer is a transfer to the same account.
var ErrSelfTransfer = errors.New("self-transfer not allowed")

// ErrAccountNotFound is a missing account on a transfer.
var ErrAccountNotFound = errors.New("account not found")

// ErrAccountMissing is a missing account on a balance read.
var ErrAccountMissing = errors.New("account not found")

// ErrTransferMissing is a missing transfer row.
var ErrTransferMissing = errors.New("transfer not found")

// ErrUnavailable is a failed Postgres connection.
var ErrUnavailable = errors.New("postgres connection failed")

// TransferService is the transfer use case.
type TransferService interface {
	PostTransfer(ctx context.Context, cmd structures.TransferCommand) (structures.Transfer, error)
	GetAccount(ctx context.Context, accountID string) (structures.Account, error)
	GetTransfer(ctx context.Context, transferID uuid.UUID) (structures.Transfer, error)
}

// Transfers performs coin transfers.
type Transfers struct {
	db       repositories.Transactor
	accounts repositories.AccountRepository
	rows     repositories.TransferRepository
}

// NewTransfers builds the transfer service.
func NewTransfers(db repositories.Transactor, accounts repositories.AccountRepository, rows repositories.TransferRepository) TransferService {
	return &Transfers{db: db, accounts: accounts, rows: rows}
}

// PostTransfer moves coins or returns the already stored outcome.
func (s *Transfers) PostTransfer(ctx context.Context, cmd structures.TransferCommand) (structures.Transfer, error) {
	stored, err := s.rows.FindModel(ctx, &models.TransferFilter{TransferID: &cmd.TransferID})
	if err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	if len(stored) > 0 {
		return replay(&stored[0], cmd)
	}
	found, err := s.accounts.FindModel(ctx, &models.AccountFilter{
		AccountID:  nil,
		AccountIDs: []string{cmd.FromAccountID, cmd.ToAccountID},
	})
	if err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	if !bothExist(found, cmd.FromAccountID, cmd.ToAccountID) {
		return structures.Transfer{}, ErrAccountNotFound
	}
	if cmd.FromAccountID == cmd.ToAccountID {
		return structures.Transfer{}, ErrSelfTransfer
	}
	return s.move(ctx, cmd)
}

// GetAccount returns the current balance.
func (s *Transfers) GetAccount(ctx context.Context, accountID string) (structures.Account, error) {
	found, err := s.accounts.FindModel(ctx, &models.AccountFilter{AccountID: &accountID, AccountIDs: nil})
	if err != nil {
		return structures.Account{}, mapStore(err)
	}
	if len(found) == 0 {
		return structures.Account{}, ErrAccountMissing
	}
	return structures.Account{AccountID: found[0].AccountID, Balance: found[0].Balance}, nil
}

// GetTransfer returns the stored transfer row.
func (s *Transfers) GetTransfer(ctx context.Context, transferID uuid.UUID) (structures.Transfer, error) {
	stored, err := s.rows.FindModel(ctx, &models.TransferFilter{TransferID: &transferID})
	if err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	if len(stored) == 0 {
		return structures.Transfer{}, ErrTransferMissing
	}
	return toTransfer(&stored[0]), nil
}

func (s *Transfers) move(ctx context.Context, cmd structures.TransferCommand) (structures.Transfer, error) {
	tx, err := s.db.BeginRepeatableRead(ctx)
	if err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	defer func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr == nil || errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return
		}
		log.Printf("rollback transfer: %v", rollbackErr)
	}()
	txCtx := repositories.WithTx(ctx, tx)
	balance, err := s.accounts.LockBalance(txCtx, cmd.FromAccountID)
	if err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	now := time.Now().UTC()
	if balance.LessThan(cmd.Amount) {
		row := newRow(cmd, models.StatusRejected, sql.NullString{String: apperrors.ErrInsufficientFunds.Reason, Valid: true}, now)
		if err = s.rows.AddModel(txCtx, &row); err != nil {
			return structures.Transfer{}, mapStore(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return structures.Transfer{}, mapStore(err)
		}
		return structures.Transfer{}, ErrInsufficientFunds
	}
	row := newRow(cmd, models.StatusCompleted, sql.NullString{}, now)
	if err = s.rows.AddModel(txCtx, &row); err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	if err = s.accounts.AddAmount(txCtx, cmd.FromAccountID, cmd.Amount.Neg()); err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	if err = s.accounts.AddAmount(txCtx, cmd.ToAccountID, cmd.Amount); err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return structures.Transfer{}, mapStore(err)
	}
	return toTransfer(&row), nil
}

func replay(row *models.TransferModel, cmd structures.TransferCommand) (structures.Transfer, error) {
	if row.FromAccountID != cmd.FromAccountID || row.ToAccountID != cmd.ToAccountID || !row.Amount.Equal(cmd.Amount) {
		return structures.Transfer{}, ErrIdempotencyConflict
	}
	if row.Status == models.StatusRejected {
		return structures.Transfer{}, ErrInsufficientFunds
	}
	return toTransfer(row), nil
}

func bothExist(rows []models.AccountModel, from, to string) bool {
	have := make(map[string]struct{})
	for i := range rows {
		have[rows[i].AccountID] = struct{}{}
	}
	_, fromOK := have[from]
	_, toOK := have[to]
	return fromOK && toOK
}

func newRow(cmd structures.TransferCommand, status string, reason sql.NullString, now time.Time) models.TransferModel {
	return models.TransferModel{
		TransferID:    cmd.TransferID,
		FromAccountID: cmd.FromAccountID,
		ToAccountID:   cmd.ToAccountID,
		Amount:        cmd.Amount,
		Status:        status,
		Reason:        reason,
		CreatedAt:     now,
	}
}

func toTransfer(row *models.TransferModel) structures.Transfer {
	reason := ""
	if row.Reason.Valid {
		reason = row.Reason.String
	}
	return structures.Transfer{
		TransferID:    row.TransferID,
		FromAccountID: row.FromAccountID,
		ToAccountID:   row.ToAccountID,
		Amount:        row.Amount,
		Status:        row.Status,
		Reason:        reason,
		CreatedAt:     row.CreatedAt,
	}
}

func mapStore(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repositories.ErrNotFound):
		return ErrAccountNotFound
	case errors.Is(err, repositories.ErrTransactionConflict):
		return ErrTransactionConflict
	case errors.Is(err, repositories.ErrTransferIDExists):
		return ErrTransferIDExists
	case errors.Is(err, repositories.ErrUnavailable):
		return ErrUnavailable
	default:
		return ErrUnavailable
	}
}
