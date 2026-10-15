// Package repositories talks to Postgres through models.
package repositories

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is a missing row inside a transfer transaction.
var ErrNotFound = errors.New("row not found")

// ErrTransactionConflict is a serialization failure or a deadlock.
var ErrTransactionConflict = errors.New("transfer transaction rolled back")

// ErrTransferIDExists is a unique transfer_id collision.
var ErrTransferIDExists = errors.New("transfer_id already exists")

// ErrUnavailable is a failed Postgres connection.
var ErrUnavailable = errors.New("postgres connection failed")

type txKey struct{}

// Transactor opens the transfer transaction.
type Transactor interface {
	BeginRepeatableRead(ctx context.Context) (pgx.Tx, error)
}

// DB is the Postgres pool.
type DB struct {
	pool *pgxpool.Pool
}

// New opens a pool.
func New(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, err.Error())
	}
	return &DB{pool: pool}, nil
}

// Close closes the pool.
func (db *DB) Close() {
	db.pool.Close()
}

// BeginRepeatableRead starts one read-write repeatable read transaction.
func (db *DB) BeginRepeatableRead(ctx context.Context) (pgx.Tx, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:       pgx.RepeatableRead,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
		BeginQuery:     "",
		CommitQuery:    "",
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return tx, nil
}

// WithTx stores the transaction for repository methods.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (db *DB) conn(ctx context.Context) querier {
	value := ctx.Value(txKey{})
	if value == nil {
		return db.pool
	}
	tx, ok := value.(pgx.Tx)
	if !ok {
		return db.pool
	}
	return tx
}

func mapErr(err error) error {
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrTransactionConflict) || errors.Is(err, ErrTransferIDExists) || errors.Is(err, ErrUnavailable) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40001", "40P01", "23514":
			return fmt.Errorf("%w", ErrTransactionConflict)
		case "23505":
			return fmt.Errorf("%w", ErrTransferIDExists)
		case "57P01", "57P02", "57P03", "53300":
			return fmt.Errorf("%w", ErrUnavailable)
		}
		if len(pgErr.Code) >= sqlStateClassLen && pgErr.Code[:sqlStateClassLen] == "08" {
			return fmt.Errorf("%w", ErrUnavailable)
		}
		return fmt.Errorf("postgres: %w", err)
	}
	var connectErr *pgconn.ConnectError
	if errors.As(err, &connectErr) {
		return fmt.Errorf("%w", ErrUnavailable)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w", ErrNotFound)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w", ErrUnavailable)
	}
	return fmt.Errorf("postgres: %w", err)
}
