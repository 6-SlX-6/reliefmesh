package database

import (
	"context"
	"fmt"
	"time"
)

// NextReference allocates the next human-readable reference for prefix in
// the current year, e.g. "RM-2026-0001042". The counter row is locked for the
// duration of the surrounding transaction, so references are unique and
// strictly increasing per prefix and year.
func NextReference(ctx context.Context, q Querier, prefix string, now time.Time) (string, error) {
	year := now.UTC().Year()
	var n int64
	err := q.QueryRow(ctx, `INSERT INTO reference_counters (prefix, year, value) VALUES ($1, $2, 1)
		ON CONFLICT (prefix, year) DO UPDATE SET value = reference_counters.value + 1
		RETURNING value`, prefix, year).Scan(&n)
	if err != nil {
		return "", fmt.Errorf("allocate reference: %w", err)
	}
	return fmt.Sprintf("%s-%d-%07d", prefix, year, n), nil
}
