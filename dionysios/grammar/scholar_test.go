package grammar

import (
	"testing"

	sv1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"github.com/stretchr/testify/assert"
)

func TestResearchResultsCanBeCappedAfterCombiningDirectAndExpandedMatches(t *testing.T) {
	response := &sv1.AnalyzeResponse{
		DirectResult: &sv1.DirectResult{Texts: []*sv1.AnalyzeResult{
			researchResult("direct-1"),
			researchResult("direct-2"),
			researchResult("direct-3"),
		}},
		Results: []*sv1.AnalyzeResult{
			researchResult("expanded-1"),
			researchResult("expanded-2"),
			researchResult("expanded-3"),
		},
	}

	results := limitResearchResults(mapResearchResults(response), 5)

	assert.Len(t, results, 5)
	assert.Equal(t, "direct-1", results[0].Reference)
	assert.Equal(t, "expanded-2", results[4].Reference)
}

func researchResult(reference string) *sv1.AnalyzeResult {
	return &sv1.AnalyzeResult{
		Reference: reference,
		Text:      &sv1.Rhema{Greek: reference},
	}
}
