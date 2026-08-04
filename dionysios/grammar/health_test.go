package grammar

import (
	"context"
	"testing"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetailedHealthReturnsCompleteUnconfiguredSnapshot(t *testing.T) {
	handler := DionysosHandler{}

	started := time.Now()
	health := handler.detailedHealth(context.Background())

	assert.Less(t, time.Since(started), 100*time.Millisecond)
	assert.False(t, health.Healthy)
	assert.Equal(t, "not_configured", health.Tracer.Status)
	assert.Equal(t, "not_configured", health.Cache.Status)
	assert.False(t, health.Mapping.Loaded)
	require.Len(t, health.Dependencies, 4)
	for _, dependency := range health.Dependencies {
		assert.False(t, dependency.Configured)
		assert.False(t, dependency.Healthy)
	}
}

func TestMappingCounts(t *testing.T) {
	handler := DionysosHandler{
		DeclensionConfig: models.DeclensionConfig{
			Declensions: []models.Declension{
				{Declensions: []models.DeclensionElement{{}, {}}},
				{Declensions: []models.DeclensionElement{{}}},
			},
		},
	}

	declensions, rules, loaded := handler.mappingCounts()

	assert.Equal(t, 2, declensions)
	assert.Equal(t, 3, rules)
	assert.True(t, loaded)
}

func TestCacheHealthWithStoredValue(t *testing.T) {
	cache, err := archytas.NewInMemoryBadgerClientWithOptions(archytas.WithLogging(false))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cache.Close()) })
	require.NoError(t, cache.Set("health-test", "something"))

	handler := DionysosHandler{Cache: cache}
	health := handler.cacheHealth()

	assert.True(t, health.Configured)
	assert.True(t, health.Healthy)
	assert.Equal(t, "healthy", health.Status)
	assert.Contains(t, health.Implementation, "archytas.Badger")
	assert.Empty(t, health.Message)
	assert.Equal(t, uint64(1), health.KeyCount)
	assert.Equal(t, health.LsmSizeBytes+health.ValueLogSizeBytes, health.TotalSizeBytes)
	assert.NotNil(t, health.BlockCache)
	assert.NotNil(t, health.IndexCache)

	stored, err := cache.Read("health-test")
	require.NoError(t, err)
	assert.Equal(t, "something", string(stored))
}
