package handlers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/diegoGarciaCo/raffles/internal/database"
)

// withTx runs fn inside a database transaction, passing a *database.Queries
// bound to that transaction. It commits on success and rolls back on error or
// panic. Use for multi-statement operations that must be atomic (e.g. checkout:
// one order + N tickets).
func (cfg *apiCfg) withTx(ctx context.Context, fn func(qtx *database.Queries) error) error {
	tx, err := cfg.RawDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		// Rollback is a no-op if the tx already committed.
		_ = tx.Rollback()
	}()

	qtx := cfg.DB.WithTx(tx)
	if err := fn(qtx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// nullInt32 is a small helper for passing optional ints to sqlc narg params.
func nullInt32(v int32, valid bool) sql.NullInt32 {
	return sql.NullInt32{Int32: v, Valid: valid}
}
