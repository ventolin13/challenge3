package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	apperrors "github.com/ventolin13/challenge3/errors"
	"github.com/ventolin13/challenge3/internal/repositories"
	"github.com/ventolin13/challenge3/models"
)

const (
	containerName       = "challenge3-pg-test"
	testDatabase        = "challenge_test"
	pgPort              = "55432"
	pgImagePort         = "5432"
	pingAttempts        = 60
	pingPause           = 500 * time.Millisecond
	startBalance        = 1000
	concurrencyAccounts = 8
	transfersPerAccount = 3
	transferChunkCap    = 20
	nextAccountOffset   = 1
	oneGoroutine        = 1
	oneAttempt          = 1
	oneRow              = 1
	retryLimit          = 20
	balanceScale        = 2
	chunkedAmount       = "400"
)

type database struct {
	db       *repositories.DB
	accounts repositories.AccountRepository
	rows     repositories.TransferRepository
}

type transferCommand struct {
	key    string
	from   string
	to     string
	amount string
}

var (
	bootOnce         sync.Once
	shared           *database
	bootErr          error
	containerStarted bool
)

func TestMain(m *testing.M) {
	code := m.Run()
	if shared != nil {
		shared.db.Close()
	}
	if containerStarted {
		if rmErr := exec.Command("docker", "rm", "-f", containerName).Run(); rmErr != nil {
			fmt.Fprintf(os.Stderr, "remove postgres container: %v\n", rmErr)
		}
	}
	os.Exit(code)
}

func TestTransfersConcurrent(t *testing.T) {
	t.Parallel()
	store := openDatabase(t)
	ids := make([]string, 0, concurrencyAccounts)
	for range concurrencyAccounts {
		ids = append(ids, addAccount(t, store, startBalance))
	}

	commands := make([]transferCommand, 0, concurrencyAccounts*transfersPerAccount)
	for i, from := range ids {
		for n := range transfersPerAccount {
			to := ids[(i+n+nextAccountOffset)%len(ids)]
			commands = append(commands, transferCommand{
				key:    uuid.NewString(),
				from:   from,
				to:     to,
				amount: chunkedAmount,
			})
		}
	}

	runTransfersChunked(t, store, transferChunkCap, commands)

	total := decimal.Zero
	for _, id := range ids {
		balance := balanceOf(t, store, id)
		require.True(t, balance.GreaterThanOrEqual(decimal.Zero))
		total = total.Add(balance)
	}
	require.Equal(t, decimal.NewFromInt(int64(concurrencyAccounts)*startBalance).StringFixed(balanceScale), total.StringFixed(balanceScale))
}

func runTransfersChunked(t *testing.T, store *database, chunkCap int, commands []transferCommand) {
	t.Helper()
	grouped := splitTransfersByFrom(commands)
	t.Logf("concurrent transfers=%d goroutines=%d chunk=%d", len(commands), len(grouped), chunkCap)
	var group sync.WaitGroup
	for from, list := range grouped {
		group.Add(oneGoroutine)
		go func(from string, list []transferCommand) {
			defer group.Done()
			t.Logf("goroutine from_account_id=%s transfers=%d", from, len(list))
			for _, chunk := range chunkTransfers(list, chunkCap) {
				t.Logf("goroutine from_account_id=%s chunk transfers=%d", from, len(chunk))
				for _, cmd := range chunk {
					if err := settleTransfer(t, store, cmd); err != nil {
						t.Errorf("from_account_id %s: %v", from, err)
					}
				}
			}
		}(from, list)
	}
	group.Wait()
	t.Logf("concurrent finished transfers=%d goroutines=%d", len(commands), len(grouped))
}

func settleTransfer(t *testing.T, store *database, cmd transferCommand) error {
	for attempt := 0; attempt < retryLimit; attempt++ {
		err := commitTransfer(t, store, cmd)
		if err == nil {
			return nil
		}
		if errors.Is(err, repositories.ErrTransactionConflict) {
			t.Logf("transaction rolled back idempotency_key=%s from_account_id=%s to_account_id=%s amount=%s attempt=%d reason=%s", cmd.key, cmd.from, cmd.to, cmd.amount, attempt+oneAttempt, err.Error())
			continue
		}
		return err
	}
	return fmt.Errorf("transfer %s stayed rolled back", cmd.key)
}

func commitTransfer(t *testing.T, store *database, cmd transferCommand) error {
	ctx := context.Background()
	tx, err := store.db.BeginRepeatableRead(ctx)
	if err != nil {
		return err
	}
	defer func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr == nil || errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return
		}
		t.Errorf("rollback transfer_id=%s: %v", cmd.key, rollbackErr)
	}()
	txCtx := repositories.WithTx(ctx, tx)
	balance, err := store.accounts.LockBalance(txCtx, cmd.from)
	if err != nil {
		return err
	}
	amount := decimal.RequireFromString(cmd.amount)
	now := time.Now().UTC()
	row := &models.TransferModel{
		TransferID:    uuid.MustParse(cmd.key),
		FromAccountID: cmd.from,
		ToAccountID:   cmd.to,
		Amount:        amount,
		Status:        models.StatusCompleted,
		Reason:        sql.NullString{},
		CreatedAt:     now,
	}
	if balance.LessThan(amount) {
		row.Status = models.StatusRejected
		row.Reason = sql.NullString{String: apperrors.ErrInsufficientFunds.Reason, Valid: true}
		if err = store.rows.AddModel(txCtx, row); err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		t.Logf("transaction committed idempotency_key=%s from_account_id=%s to_account_id=%s amount=%s status=rejected reason=%s", cmd.key, cmd.from, cmd.to, cmd.amount, row.Reason.String)
		return nil
	}
	if err = store.rows.AddModel(txCtx, row); err != nil {
		return err
	}
	if err = store.accounts.AddAmount(txCtx, cmd.from, amount.Neg()); err != nil {
		return err
	}
	if err = store.accounts.AddAmount(txCtx, cmd.to, amount); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	t.Logf("transaction committed idempotency_key=%s from_account_id=%s to_account_id=%s amount=%s status=completed", cmd.key, cmd.from, cmd.to, cmd.amount)
	return nil
}

func splitTransfersByFrom(commands []transferCommand) map[string][]transferCommand {
	grouped := make(map[string][]transferCommand)
	for _, cmd := range commands {
		grouped[cmd.from] = append(grouped[cmd.from], cmd)
	}
	return grouped
}

func chunkTransfers(commands []transferCommand, chunkCap int) [][]transferCommand {
	batch := make([][]transferCommand, 0, oneGoroutine+len(commands)/chunkCap)
	var items []transferCommand
	for _, cmd := range commands {
		items = append(items, cmd)
		if len(items) == chunkCap {
			batch = append(batch, items)
			items = nil
		}
	}
	if items != nil {
		batch = append(batch, items)
	}
	return batch
}

func openDatabase(t *testing.T) *database {
	t.Helper()
	bootOnce.Do(boot)
	if bootErr != nil {
		t.Fatal(bootErr)
	}
	return shared
}

func boot() {
	url := startPostgres()
	if bootErr != nil {
		return
	}
	bootErr = connect(url)
}

func startPostgres() string {
	out, rmErr := exec.Command("docker", "rm", "-f", containerName).CombinedOutput()
	if rmErr != nil && !strings.Contains(string(out), "No such container") {
		bootErr = fmt.Errorf("remove old postgres container: %s: %w", out, rmErr)
		return ""
	}
	out, err := exec.Command(
		"docker", "run", "-d",
		"--name", containerName,
		"-e", "POSTGRES_USER=challenge",
		"-e", "POSTGRES_PASSWORD=challenge",
		"-e", "POSTGRES_DB="+testDatabase,
		"-p", pgPort+":"+pgImagePort,
		"postgres:16",
	).CombinedOutput()
	if err != nil {
		bootErr = fmt.Errorf("postgres container: %s: %w", out, err)
		return ""
	}
	containerStarted = true
	return "postgres://challenge:challenge@127.0.0.1:" + pgPort + "/" + testDatabase + "?sslmode=disable"
}

func connect(url string) error {
	ctx := context.Background()
	pool, err := waitPostgres(ctx, url)
	if err != nil {
		return err
	}
	if err = applySQL(ctx, pool, migrationsDir()); err != nil {
		pool.Close()
		return err
	}
	pool.Close()

	db, err := repositories.New(ctx, url)
	if err != nil {
		return err
	}
	shared = &database{
		db:       db,
		accounts: repositories.NewAccounts(db),
		rows:     repositories.NewTransfers(db),
	}
	return nil
}

func waitPostgres(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var err error
	for attempt := 0; attempt < pingAttempts; attempt++ {
		var pool *pgxpool.Pool
		pool, err = pgxpool.New(ctx, url)
		if err == nil {
			err = pool.Ping(ctx)
			if err == nil {
				return pool, nil
			}
			pool.Close()
		}
		time.Sleep(pingPause)
	}
	return nil, fmt.Errorf("postgres connection failed: %w", err)
}

func applySQL(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "V") || !strings.HasSuffix(name, ".sql") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil {
			return readErr
		}
		if _, execErr := pool.Exec(ctx, string(body)); execErr != nil {
			return execErr
		}
	}
	return nil
}

func migrationsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "migrations"
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func addAccount(t *testing.T, store *database, balance int64) string {
	t.Helper()
	id := "acc_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	err := store.accounts.AddModel(context.Background(), &models.AccountModel{
		AccountID: id,
		Balance:   decimal.NewFromInt(balance),
	})
	require.NoError(t, err)
	return id
}

func balanceOf(t *testing.T, store *database, accountID string) decimal.Decimal {
	t.Helper()
	found, err := store.accounts.FindModel(context.Background(), &models.AccountFilter{
		AccountID:  &accountID,
		AccountIDs: nil,
	})
	require.NoError(t, err)
	require.Len(t, found, oneRow)
	return found[0].Balance
}
