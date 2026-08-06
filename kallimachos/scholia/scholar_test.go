package scholia

import (
	"context"
	"strings"
	"testing"

	elastic "github.com/odysseia-greek/agora/aristoteles"
	esmodels "github.com/odysseia-greek/agora/aristoteles/models"
	v1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	testScholarIndex = "kallimachos-test"
	directTextHit    = `{
		"took": 4,
		"timed_out": false,
		"_shards": {
			"total": 1,
			"successful": 1,
			"skipped": 0,
			"failed": 0
		},
		"hits": {
			"total": {
				"value": 1,
				"relation": "eq"
			},
			"max_score": 1,
			"hits": [
				{
					"_index": "kallimachos-test",
					"_type": "_doc",
					"_id": "text-1",
					"_score": 1,
					"_source": {
						"author": "Plato",
						"book": "Republic",
						"reference": "1.1",
						"rhemai": [
							{
								"greek": "ὁ λόγος καλός",
								"translations": ["the word is noble"],
								"section": "1"
							}
						]
					}
				}
			]
		}
	}`
	expandedTextHit = `{
		"took": 5,
		"timed_out": false,
		"_shards": {
			"total": 1,
			"successful": 1,
			"skipped": 0,
			"failed": 0
		},
		"hits": {
			"total": {
				"value": 1,
				"relation": "eq"
			},
			"max_score": 1,
			"hits": [
				{
					"_index": "kallimachos-test",
					"_type": "_doc",
					"_id": "text-2",
					"_score": 1,
					"_source": {
						"author": "Herodotos",
						"book": "Histories",
						"reference": "2.3",
						"rhemai": [
							{
								"greek": "λέγει τὸν λόγον",
								"translations": ["he speaks the account"],
								"section": "2"
							}
						]
					}
				}
			]
		}
	}`
	emptyTextHit = `{
		"took": 1,
		"timed_out": false,
		"_shards": {
			"total": 1,
			"successful": 1,
			"skipped": 0,
			"failed": 0
		},
		"hits": {
			"total": {
				"value": 0,
				"relation": "eq"
			},
			"max_score": 0,
			"hits": []
		}
	}`
)

func TestAnalyzeReturnsDirectAndExpandedResults(t *testing.T) {
	service := newTestScholarService(t, directTextHit, expandedTextHit)

	response, err := service.Analyze(context.Background(), newAnalyzeRequest())

	assert.Nil(t, err)
	assert.Equal(t, "λόγος", response.Rootword)
	assert.Equal(t, "NOUN", response.PartOfSpeech)
	assert.Len(t, response.Conjugations, 2)
	assert.Equal(t, "λόγος", response.DirectResult.RequestedWord)
	assert.Len(t, response.DirectResult.Texts, 1)
	assert.Contains(t, response.DirectResult.Texts[0].Text.Greek, "&&&λόγος&&&")
	assert.Len(t, response.Results, 1)
	assert.Contains(t, response.Results[0].Text.Greek, "&&&λόγον&&&")
}

func TestAnalyzeRequiresRootword(t *testing.T) {
	service := ScholarServiceImpl{}

	response, err := service.Analyze(context.Background(), &v1.AnalyzeRequest{Rootword: "   "})

	assert.Nil(t, response)
	assert.EqualError(t, err, "rootword is required")
}

func TestFindTextReturnsMatchingSection(t *testing.T) {
	service := newTestScholarService(t, directTextHit)

	response, err := service.FindText(context.Background(), &v1.FindTextRequest{
		Text:  "ὁ λόγος καλός.",
		Limit: 5,
	})

	assert.NoError(t, err)
	assert.True(t, response.Found)
	assert.Equal(t, uint32(1), response.MatchCount)
	assert.Len(t, response.Matches, 1)
	assert.Equal(t, "Plato", response.Matches[0].Author)
	assert.Equal(t, "Republic", response.Matches[0].Book)
	assert.Equal(t, "ὁ λόγος καλός", response.Matches[0].Text.Greek)
	assert.Contains(t, response.Message, "original phrase")
}

func TestFindTextReturnsFoundFalseWhenPhraseIsAbsent(t *testing.T) {
	service := newTestScholarService(t, emptyTextHit, emptyTextHit, emptyTextHit)

	response, err := service.FindText(context.Background(), &v1.FindTextRequest{Text: "τοῦτο τὸ χωρίον οὐκ ἔστιν"})

	assert.NoError(t, err)
	assert.False(t, response.Found)
	assert.Zero(t, response.MatchCount)
	assert.Empty(t, response.Matches)
	assert.Equal(t, "no match found after trying: original phrase, 4-word windows, 3-word windows", response.Message)
}

func TestFindTextRequiresText(t *testing.T) {
	response, err := (&ScholarServiceImpl{}).FindText(context.Background(), &v1.FindTextRequest{Text: "   "})

	assert.Nil(t, response)
	assert.Equal(t, "text is required", status.Convert(err).Message())
}

func TestCreateFindTextQueryUsesNestedPhraseMatch(t *testing.T) {
	query := createFindTextQuery([]string{"ἀρχὴ πάσης", "πάσης πράξεως"}, 5)
	nested := query["query"].(map[string]interface{})["nested"].(map[string]interface{})
	boolQuery := nested["query"].(map[string]interface{})["bool"].(map[string]interface{})
	phrases := boolQuery["should"].([]map[string]interface{})

	assert.Equal(t, "rhemai", nested["path"])
	assert.Equal(t, "ἀρχὴ πάσης", phrases[0]["match_phrase"].(map[string]interface{})["rhemai.greek"])
	assert.Equal(t, "πάσης πράξεως", phrases[1]["match_phrase"].(map[string]interface{})["rhemai.greek"])
	assert.Equal(t, 1, boolQuery["minimum_should_match"])
	assert.Equal(t, uint32(5), query["size"])
}

func TestFindTextUsesNormalizedPhraseFallback(t *testing.T) {
	service := newTestScholarService(t, emptyTextHit, directTextHit)

	response, err := service.FindText(context.Background(), &v1.FindTextRequest{Text: "ὁ λόγος καλός!"})

	assert.NoError(t, err)
	assert.True(t, response.Found)
	assert.Contains(t, response.Message, "punctuation-normalized phrase")
}

func TestFindTextRejectsTextOutsideWordBounds(t *testing.T) {
	tests := []struct {
		text    string
		message string
	}{
		{text: "λόγος καλός", message: "text must contain at least 3 words; use Analyze for shorter searches"},
		{text: strings.Repeat("λόγος ", 51), message: "text must contain at most 50 words"},
	}

	for _, test := range tests {
		response, err := (&ScholarServiceImpl{}).FindText(context.Background(), &v1.FindTextRequest{Text: test.text})
		assert.Nil(t, response)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Equal(t, test.message, status.Convert(err).Message())
	}
}

func TestFallbackWindowSizesScaleWithTextLength(t *testing.T) {
	assert.Equal(t, []int{4, 3}, fallbackWindowSizes(5))
	assert.Equal(t, []int{10, 5}, fallbackWindowSizes(39))
}

func TestTextWindowsUsesContiguousSlidingPhrases(t *testing.T) {
	windows := textWindows([]string{"one", "two", "three", "four", "five"}, 4)
	assert.Equal(t, []string{"one two three four", "two three four five"}, windows)
}

func TestAnalyzeReturnsDirectOnlyWhenNoExpandedWords(t *testing.T) {
	service := newTestScholarService(t, directTextHit)

	response, err := service.Analyze(context.Background(), &v1.AnalyzeRequest{
		Rootword: "λόγος",
		Limit:    5,
		Conjugations: []*v1.Conjugation{
			{Word: "λόγος", Rule: "noun - sing - masc - nom"},
		},
	})

	assert.Nil(t, err)
	assert.Len(t, response.DirectResult.Texts, 1)
	assert.Empty(t, response.Results)
}

func TestAnalyzeReturnsErrorWhenNoHits(t *testing.T) {
	service := newTestScholarService(t, emptyTextHit, emptyTextHit)

	response, err := service.Analyze(context.Background(), newAnalyzeRequest())

	assert.Nil(t, response)
	assert.EqualError(t, err, `no hits for rootword "λόγος"`)
}

func TestAnalyzeWordsFallsBackToRootword(t *testing.T) {
	words := analyzeWords("λόγος", nil)
	assert.Equal(t, []string{"λόγος"}, words)
}

func TestAnalyzeWordsDeduplicatesWordsAndAddsMissingRoot(t *testing.T) {
	words := analyzeWords("λόγος", []*v1.Conjugation{
		{Word: "λόγον", Rule: "noun - sing - masc - acc"},
		{Word: "λόγον", Rule: "duplicate"},
	})
	assert.ElementsMatch(t, []string{"λόγον", "λόγος"}, words)
}

func TestQueryTextsReturnsHitsTotal(t *testing.T) {
	service := newTestScholarService(t, directTextHit)

	response, hits, err := service.queryTexts(context.Background(), createGreekTextQuery([]string{"λόγος"}, 5))

	assert.Nil(t, err)
	assert.Equal(t, int64(1), hits)
	assert.Len(t, response.Hits.Hits, 1)
}

func TestCollectAnalyzeResultsHonorsLimit(t *testing.T) {
	response := &esmodels.Response{
		Hits: esmodels.Hits{
			Hits: []esmodels.Hit{
				{Source: map[string]interface{}{
					"author":    "Plato",
					"book":      "Republic",
					"reference": "1.1",
					"rhemai": []interface{}{
						map[string]interface{}{"greek": "λόγος λόγον", "translations": []interface{}{"one"}, "section": "1"},
						map[string]interface{}{"greek": "λόγον", "translations": []interface{}{"two"}, "section": "2"},
					},
				}},
			},
		},
	}

	results, err := collectAnalyzeResults(response, []string{"λόγος", "λόγον"}, 1)

	assert.Nil(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, results[0].Text.Greek, "&&&λόγος&&&")
}

func TestCollectAnalyzeResultsHandlesNilResponse(t *testing.T) {
	results, err := collectAnalyzeResults(nil, []string{"λόγος"}, 5)

	assert.Nil(t, err)
	assert.Empty(t, results)
}

func TestDecodeTextDocument(t *testing.T) {
	text, err := decodeTextDocument(map[string]interface{}{
		"author":    "Plato",
		"book":      "Republic",
		"reference": "1.1",
		"rhemai": []interface{}{
			map[string]interface{}{"greek": "λόγος", "translations": []interface{}{"word"}, "section": "1"},
		},
	})

	assert.Nil(t, err)
	assert.Equal(t, "Plato", text.Author)
	assert.Equal(t, "λόγος", text.Rhemai[0].Greek)
}

func TestAnalyzeTextSectionsHighlightsMatchingSections(t *testing.T) {
	text := TextDocument{
		Author:    "Plato",
		Book:      "Republic",
		Reference: "1.1",
		Rhemai: []Rhema{
			{Greek: "λόγος ἐστίν", Translations: []string{"it is a word"}, Section: "1"},
			{Greek: "ἀρετή ἐστίν", Translations: []string{"it is virtue"}, Section: "2"},
		},
	}

	results := analyzeTextSections(text, []string{"λόγος"})

	assert.Len(t, results, 1)
	assert.Equal(t, "/texts?author=Plato&book=Republic&reference=1.1", results[0].ReferenceLink)
	assert.Contains(t, results[0].Text.Greek, "&&&λόγος&&&")
}

func TestHighlightSectionDeduplicatesAndPrefersLongerWords(t *testing.T) {
	section := Rhema{Greek: "λόγος λόγον"}

	highlighted, found := highlightSection(section, []string{"λόγ", "λόγος", "λόγος"})

	assert.True(t, found)
	assert.Contains(t, highlighted.Greek, "&&&λόγος&&&")
}

func TestUniqueWords(t *testing.T) {
	assert.ElementsMatch(t, []string{"λόγος", "λόγον"}, uniqueWords([]string{"λόγος", " ", "λόγον", "λόγος"}))
}

func TestExpandedWordsWithoutRoot(t *testing.T) {
	assert.ElementsMatch(t, []string{"λόγον"}, expandedWordsWithoutRoot([]string{"λόγος", "λόγον", "λόγον"}, "λόγος"))
}

func TestBuildReferenceLinkEscapesValues(t *testing.T) {
	assert.Equal(t, "/texts?author=Plato+Atheniensis&book=Republic&reference=1.1", buildReferenceLink("Plato Atheniensis", "Republic", "1.1"))
}

func TestCreateGreekTextQuery(t *testing.T) {
	query := createGreekTextQuery([]string{"λόγος", "λόγος", "λόγον"}, 7)

	assert.Equal(t, uint32(7), query["size"])

	boolQuery := query["query"].(map[string]interface{})["bool"].(map[string]interface{})
	should := boolQuery["should"].([]map[string]interface{})
	assert.Len(t, should, 2)
}

func TestSanitizeLimit(t *testing.T) {
	assert.Equal(t, uint32(5), sanitizeLimit(0))
	assert.Equal(t, uint32(12), sanitizeLimit(12))
	assert.Equal(t, uint32(20), sanitizeLimit(200))
}

func newAnalyzeRequest() *v1.AnalyzeRequest {
	return &v1.AnalyzeRequest{
		Rootword:     "λόγος",
		Limit:        5,
		PartOfSpeech: "NOUN",
		Conjugations: []*v1.Conjugation{
			{Word: "λόγος", Rule: "noun - sing - masc - nom"},
			{Word: "λόγον", Rule: "noun - sing - masc - acc"},
		},
	}
}

func newTestScholarService(t *testing.T, fixtures ...string) *ScholarServiceImpl {
	t.Helper()

	rawFixtures := make([][]byte, 0, len(fixtures))
	for _, fixture := range fixtures {
		rawFixtures = append(rawFixtures, []byte(fixture))
	}

	mockElasticClient, err := elastic.NewMockClient(rawFixtures, 200)
	assert.Nil(t, err)

	return &ScholarServiceImpl{
		Elastic: mockElasticClient,
		Index:   testScholarIndex,
	}
}
