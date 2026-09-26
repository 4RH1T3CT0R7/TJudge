//go:build integration

package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// партиции старше срока хранения удаляются, текущая и с нестандартным именем остаются
func TestDB_DropOldPartitions(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	for _, ddl := range []string{
		"CREATE TABLE matches_2000_01 PARTITION OF matches FOR VALUES FROM ('2000-01-01') TO ('2000-02-01')",
		"CREATE TABLE matches_archive PARTITION OF matches FOR VALUES FROM ('2000-02-01') TO ('2000-03-01')",
	} {
		_, err := database.ExecContext(ctx, ddl)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), "DROP TABLE IF EXISTS matches_2000_01, matches_archive")
	})
	require.NoError(t, database.EnsureMatchPartitions(ctx))

	exists := func(name string) bool {
		var ok bool
		require.NoError(t, database.GetContext(ctx, &ok, "SELECT to_regclass($1) IS NOT NULL", name))
		return ok
	}

	// retention выключен
	require.NoError(t, database.DropOldPartitions(ctx, 0))
	assert.True(t, exists("matches_2000_01"))

	require.NoError(t, database.DropOldPartitions(ctx, 24))
	assert.False(t, exists("matches_2000_01"))
	assert.True(t, exists("matches_archive"))
	assert.True(t, exists(fmt.Sprintf("matches_%s", time.Now().UTC().Format("2006_01"))))
}
