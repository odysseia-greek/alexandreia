package scholia

import (
	"context"
	"fmt"
	"testing"

	elastic "github.com/odysseia-greek/agora/aristoteles"
	esmodels "github.com/odysseia-greek/agora/aristoteles/models"
	ariv1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	v1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"github.com/stretchr/testify/assert"
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

type fakeAggregatorResolver struct {
	entry *ariv1.RootWordResponse
	err   error
}

func (f *fakeAggregatorResolver) RetrieveEntry(ctx context.Context, request *ariv1.AggregatorRequest) (*ariv1.RootWordResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entry, nil
}

func TestAnalyzeReturnsDirectAndExpandedResults(t *testing.T) {
	service := newTestScholarService(t, newRootWordEntry(), directTextHit, expandedTextHit)

	response, err := service.Analyze(context.Background(), &v1.AnalyzeRequest{Rootword: "λόγος", Limit: 5})

	assert.Nil(t, err)
	assert.Equal(t, "λόγος", response.Rootword)
	assert.Equal(t, ariv1.PartOfSpeech_NOUN.String(), response.PartOfSpeech)
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

func TestAnalyzeReturnsDirectOnlyWhenNoExpandedWords(t *testing.T) {
	entry := &ariv1.RootWordResponse{
		RootWord:     "λόγος",
		PartOfSpeech: ariv1.PartOfSpeech_NOUN,
		Categories: []*ariv1.GrammaticalCategory{
			{Forms: []*ariv1.GrammaticalForm{{Word: "λόγος", Rule: "noun - sing - masc - nom"}}},
		},
	}
	service := newTestScholarService(t, entry, directTextHit)

	response, err := service.Analyze(context.Background(), &v1.AnalyzeRequest{Rootword: "λόγος", Limit: 5})

	assert.Nil(t, err)
	assert.Len(t, response.DirectResult.Texts, 1)
	assert.Empty(t, response.Results)
}

func TestAnalyzeReturnsErrorWhenNoHits(t *testing.T) {
	service := newTestScholarService(t, newRootWordEntry(), emptyTextHit, emptyTextHit)

	response, err := service.Analyze(context.Background(), &v1.AnalyzeRequest{Rootword: "λόγος", Limit: 5})

	assert.Nil(t, response)
	assert.EqualError(t, err, `no hits for rootword "λόγος"`)
}

func TestResolveFormsFallsBackToRootwordWhenAggregatorFails(t *testing.T) {
	service := ScholarServiceImpl{
		Aggregator: &fakeAggregatorResolver{err: fmt.Errorf("not found")},
	}

	entry, words, conjugations := service.resolveForms(context.Background(), "λόγος")

	assert.Nil(t, entry)
	assert.ElementsMatch(t, []string{"λόγος"}, words)
	assert.Nil(t, conjugations)
}

func TestResolveFormsDeduplicatesWordsAndAddsMissingRoot(t *testing.T) {
	entry := &ariv1.RootWordResponse{
		RootWord: "λόγος",
		Categories: []*ariv1.GrammaticalCategory{
			{Forms: []*ariv1.GrammaticalForm{
				{Word: "λόγον", Rule: "noun - sing - masc - acc"},
				{Word: "λόγον", Rule: "duplicate"},
			}},
		},
	}
	service := ScholarServiceImpl{Aggregator: &fakeAggregatorResolver{entry: entry}}

	resolvedEntry, words, conjugations := service.resolveForms(context.Background(), "λόγος")

	assert.Equal(t, entry, resolvedEntry)
	assert.ElementsMatch(t, []string{"λόγον", "λόγος"}, words)
	assert.Len(t, conjugations, 2)
}

func TestQueryTextsReturnsHitsTotal(t *testing.T) {
	service := newTestScholarService(t, newRootWordEntry(), directTextHit)

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

func newRootWordEntry() *ariv1.RootWordResponse {
	return &ariv1.RootWordResponse{
		RootWord:     "λόγος",
		PartOfSpeech: ariv1.PartOfSpeech_NOUN,
		Categories: []*ariv1.GrammaticalCategory{
			{Forms: []*ariv1.GrammaticalForm{
				{Word: "λόγος", Rule: "noun - sing - masc - nom"},
				{Word: "λόγον", Rule: "noun - sing - masc - acc"},
			}},
		},
	}
}

func newTestScholarService(t *testing.T, entry *ariv1.RootWordResponse, fixtures ...string) *ScholarServiceImpl {
	t.Helper()

	rawFixtures := make([][]byte, 0, len(fixtures))
	for _, fixture := range fixtures {
		rawFixtures = append(rawFixtures, []byte(fixture))
	}

	mockElasticClient, err := elastic.NewMockClient(rawFixtures, 200)
	assert.Nil(t, err)

	return &ScholarServiceImpl{
		Elastic:    mockElasticClient,
		Index:      testScholarIndex,
		Aggregator: &fakeAggregatorResolver{entry: entry},
	}
}
