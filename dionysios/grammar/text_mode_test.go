package grammar

import (
	"context"
	"testing"
	"time"

	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	sv1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTokenizeTextRemovesPunctuationAndPreservesOrder(t *testing.T) {
	words := tokenizeText("Ἀρχὴ πάσης πράξεως ἐστὶν, ἡ τοῦ αἱρεῖσθαι ἀρχή.")

	assert.Equal(t, []string{"Ἀρχὴ", "πάσης", "πράξεως", "ἐστὶν", "ἡ", "τοῦ", "αἱρεῖσθαι", "ἀρχή"}, words)
}

func TestGrammarCacheKeySharesCapitalizedAndLowercaseAnalysis(t *testing.T) {
	assert.Equal(t, grammarCacheKey("ἀρχὴ"), grammarCacheKey("Ἀρχὴ"))
	assert.Equal(t, "grammar:v2:ἀρχὴ", grammarCacheKey("Ἀρχὴ"))
}

func TestBestLiteralGlossPrefersExactLemmaAndCompactsDictionarySense(t *testing.T) {
	results := []*v1.DeclensionResult{
		{RootWord: "ἄρχος", Rule: "noun - sing - masc - nom", Translation: []string{"a leader, chief, commander; the rectum, anus"}},
		{RootWord: "ἀρχή", Rule: "noun - sing - fem - nom", Translation: []string{"a beginning, rule, office, empire"}},
	}

	assert.Equal(t, "beginning", bestLiteralGloss("Ἀρχὴ", results))
}

func TestBestLiteralGlossPrefersArticleForAmbiguousEta(t *testing.T) {
	results := []*v1.DeclensionResult{
		{RootWord: "εἰμί", Rule: "verb - 3rd sing", Translation: []string{"to be"}},
		{RootWord: "ὁ", Rule: "article - sing - fem - nom", Translation: []string{"[feminine article nom sg]"}},
	}

	assert.Equal(t, "the", bestLiteralGloss("ἡ", results))
}

func TestBestLiteralGlossDoesNotTreatParticlesAsArticles(t *testing.T) {
	tests := []struct {
		token       string
		translation string
		expected    string
	}{
		{token: "μὲν", translation: "on the one hand (paired with δέ)", expected: "on the one hand (paired with δέ)"},
		{token: "δὲ", translation: "but; however", expected: "but"},
	}

	for _, test := range tests {
		t.Run(test.token, func(t *testing.T) {
			results := []*v1.DeclensionResult{{
				RootWord:    test.token,
				Rule:        "particle",
				Translation: []string{test.translation},
			}}

			assert.Equal(t, test.expected, bestLiteralGloss(test.token, results))
		})
	}
}

func TestTextModeRateLimitsSessionAndUpstreamIP(t *testing.T) {
	handler := &DionysosHandler{}
	window := time.Minute

	_, err := handler.reserveTextModeRequest("session-1", "127.0.0.1", window)
	require.NoError(t, err)

	_, err = handler.reserveTextModeRequest("session-1", "127.0.0.2", window)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))

	_, err = handler.reserveTextModeRequest("session-2", "127.0.0.1", window)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestFindKnownTextSkipsQueriesOutsideKallimachosBounds(t *testing.T) {
	handler := &DionysosHandler{}

	result := handler.findKnownText(context.Background(), "λόγος", 1)

	assert.False(t, result.Searched)
	assert.Equal(t, "skipped", result.Status)
	assert.Contains(t, result.Message, "3 and 50 words")
}

func TestMapKnownTextResponsePreservesSearchEvidence(t *testing.T) {
	response := &sv1.FindTextResponse{
		Query:      "τε καὶ θωμαστά",
		Found:      true,
		MatchCount: 1,
		Message:    "match found using original phrase after trying: original phrase",
		Matches: []*sv1.AnalyzeResult{{
			Author: "Herodotus",
			Book:   "Histories",
			Text:   &sv1.Rhema{Greek: "ἔργα μεγάλα τε καὶ θωμαστά"},
		}},
	}

	result := mapKnownTextResponse(response.Query, response)

	assert.True(t, result.Searched)
	assert.True(t, result.Found)
	assert.Equal(t, "found", result.Status)
	assert.Equal(t, uint32(1), result.MatchCount)
	assert.Equal(t, "Herodotus", result.Matches[0].Author)
	assert.Equal(t, response.Message, result.Message)
}
