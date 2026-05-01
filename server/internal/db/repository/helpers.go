// Package repository contains SQLC-generated code plus hand-written helpers.
// Do NOT edit the generated files (db.go, models.go, *.sql.go) — run `make sqlc-gen` instead.
package repository

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// StringToUUID converts a UUID string to a pgtype.UUID suitable for SQLC queries.
func StringToUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid UUID %q: %w", s, err)
	}
	return u, nil
}
