package grammar

import (
	"context"
	"os"
	"testing"

	elastic "github.com/odysseia-greek/agora/aristoteles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateElasticConfig(t *testing.T) {
	t.Run("UsingLocal", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping testing in short mode")
		}
		os.Setenv("ENV", "development")

		expected := "attic"
		fixtureFile := "declensionsDionysos"
		mockCode := 200
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)

		declensionConfig, err := QueryRuleSet(t.Context(), mockElasticClient, "dionysios")
		assert.NotNil(t, declensionConfig)
		assert.Nil(t, err)

		os.Setenv("ENV", "")

		assert.Equal(t, declensionConfig.Declensions[0].Dialect, expected)
		assert.Equal(t, declensionConfig.Declensions[1].Dialect, expected)
	})

	t.Run("UsingElastic", func(t *testing.T) {
		os.Setenv("ENV", "somethingelse")

		expected := "attic"
		fixtureFile := "declensionsDionysos"
		mockCode := 200
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)

		declensionConfig, err := QueryRuleSet(t.Context(), mockElasticClient, "dionysios")
		assert.NotNil(t, declensionConfig)
		assert.Nil(t, err)

		os.Setenv("ENV", "")

		assert.Equal(t, declensionConfig.Declensions[0].Dialect, expected)
		assert.Equal(t, declensionConfig.Declensions[1].Dialect, expected)
	})

	t.Run("UsingElasticWithError", func(t *testing.T) {
		os.Setenv("ENV", "somethingelse")

		fixtureFile := "malformed"
		mockCode := 200
		mockElasticClient, err := elastic.NewMockClient(fixtureFile, mockCode)
		assert.Nil(t, err)

		declensionConfig, err := QueryRuleSet(t.Context(), mockElasticClient, "dionysios")
		assert.Nil(t, declensionConfig)
		assert.NotNil(t, err)

		os.Setenv("ENV", "")
	})
}

func TestQueryRuleSetHonorsCancellation(t *testing.T) {
	mockElasticClient, err := elastic.NewMockClient("declensionsDionysos", 200)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	declensionConfig, err := QueryRuleSet(ctx, mockElasticClient, "dionysios")

	assert.Nil(t, declensionConfig)
	assert.ErrorIs(t, err, context.Canceled)
}
