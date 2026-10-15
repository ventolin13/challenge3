package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apperrors "github.com/ventolin13/challenge3/errors"
	"github.com/ventolin13/challenge3/internal/services"
	"github.com/ventolin13/challenge3/internal/services/structures"
	repomocks "github.com/ventolin13/challenge3/mocks/internal_/repositories"
	"github.com/ventolin13/challenge3/models"
)

const (
	fromAccountID = "acc_1"
	toAccountID   = "acc_2"
	fromBalance   = 5
	moveAmount    = 10
	transferKey   = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
)

func TestPostTransferStoresInsufficientFunds(t *testing.T) {
	t.Parallel()
	db := repomocks.NewMockTransactor(t)
	accounts := repomocks.NewMockAccountRepository(t)
	rows := repomocks.NewMockTransferRepository(t)
	db.EXPECT().BeginRepeatableRead(mock.Anything).Return(repeatableReadTx{}, nil)
	rows.EXPECT().FindModel(mock.Anything, mock.Anything).Return([]models.TransferModel{}, nil)
	accounts.EXPECT().FindModel(mock.Anything, mock.Anything).Return([]models.AccountModel{
		{AccountID: fromAccountID, Balance: decimal.NewFromInt(fromBalance)},
		{AccountID: toAccountID, Balance: decimal.NewFromInt(moveAmount)},
	}, nil)
	accounts.EXPECT().LockBalance(mock.Anything, fromAccountID).Return(decimal.NewFromInt(fromBalance), nil)
	rows.EXPECT().AddModel(mock.Anything, mock.MatchedBy(func(row *models.TransferModel) bool {
		return row.FromAccountID == fromAccountID &&
			row.ToAccountID == toAccountID &&
			row.Amount.Equal(decimal.NewFromInt(moveAmount)) &&
			row.Status == models.StatusRejected &&
			row.Reason.String == apperrors.ErrInsufficientFunds.Reason
	})).Return(nil)

	_, err := services.NewTransfers(db, accounts, rows).PostTransfer(context.Background(), structures.TransferCommand{
		TransferID:    uuid.MustParse(transferKey),
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        decimal.NewFromInt(moveAmount),
	})
	require.ErrorIs(t, err, services.ErrInsufficientFunds)
}

type repeatableReadTx struct{}

func (repeatableReadTx) Begin(context.Context) (pgx.Tx, error) {
	panic("begin")
}

func (repeatableReadTx) Commit(context.Context) error {
	return nil
}

func (repeatableReadTx) Rollback(context.Context) error {
	return pgx.ErrTxClosed
}

func (repeatableReadTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	panic("copy from")
}

func (repeatableReadTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	panic("send batch")
}

func (repeatableReadTx) LargeObjects() pgx.LargeObjects {
	panic("large objects")
}

func (repeatableReadTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	panic("prepare")
}

func (repeatableReadTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("exec")
}

func (repeatableReadTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("query")
}

func (repeatableReadTx) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("query row")
}

func (repeatableReadTx) Conn() *pgx.Conn {
	panic("conn")
}
