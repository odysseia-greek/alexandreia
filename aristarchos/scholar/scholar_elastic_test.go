package scholar

import (
	"context"
	"testing"

	elastic "github.com/odysseia-greek/agora/aristoteles"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/stretchr/testify/assert"
)

const (
	testScholarIndex = "aristarchos-test"
	createDocument   = `{
		"_index": "aristarchos-test",
		"_type": "_doc",
		"_id": "created-root-word",
		"_version": 1,
		"result": "created",
		"_shards": {
			"total": 1,
			"successful": 1,
			"failed": 0
		},
		"_seq_no": 1,
		"_primary_term": 1
	}`
	updateDocument = `{
		"_index": "aristarchos-test",
		"_type": "_doc",
		"_id": "root-word-1",
		"_version": 2,
		"result": "updated",
		"_shards": {
			"total": 1,
			"successful": 1,
			"failed": 0
		},
		"_seq_no": 2,
		"_primary_term": 1
	}`
	rootWordHit = `{
		"took": 7,
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
			"max_score": 4.2,
			"hits": [
				{
					"_index": "aristarchos-test",
					"_type": "_doc",
					"_id": "root-word-1",
					"_score": 4.2,
					"_source": {
						"rootWord": "λόγος",
						"partOfSpeech": "noun",
						"translations": ["word", "account"],
						"unaccented": "λογος",
						"variants": [
							{
								"searchTerm": "λόγος",
								"score": 3
							}
						],
						"categories": [
							{
								"forms": [
									{
										"word": "λόγος",
										"rule": "noun - sing - masc - nom"
									},
									{
										"word": "λόγον",
										"rule": "noun - sing - masc - acc"
									}
								]
							}
						]
					}
				}
			]
		}
	}`
	emptyHit = `{
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

func TestAggregatorServiceImplRetrieveEntryUsesElasticMock(t *testing.T) {
	service := newTestAggregatorService(t, rootWordHit)

	response, err := service.RetrieveEntry(context.Background(), &v1.AggregatorRequest{RootWord: "λόγος"})

	assert.Nil(t, err)
	assert.Equal(t, "λόγος", response.RootWord)
	assert.Equal(t, v1.PartOfSpeech_NOUN, response.PartOfSpeech)
	assert.ElementsMatch(t, []string{"word", "account"}, response.Translations)
	assert.Len(t, response.Categories, 1)
	assert.Len(t, response.Categories[0].Forms, 2)
	assert.Equal(t, "λόγος", response.Categories[0].Forms[0].Word)
	assert.Equal(t, "noun - sing - masc - nom", response.Categories[0].Forms[0].Rule)
}

func TestAggregatorServiceImplRetrieveEntryNoHits(t *testing.T) {
	service := newTestAggregatorService(t, emptyHit)

	response, err := service.RetrieveEntry(context.Background(), &v1.AggregatorRequest{RootWord: "ἄγνωστος"})

	assert.Nil(t, response)
	assert.EqualError(t, err, "no entry can be found")
}

func TestAggregatorServiceImplRetrieveSearchWordsUsesElasticMock(t *testing.T) {
	service := newTestAggregatorService(t, rootWordHit)

	response, err := service.RetrieveSearchWords(context.Background(), &v1.AggregatorRequest{RootWord: "λογ"})

	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{"λόγος", "λόγον"}, response.Word)
}

func TestAggregatorServiceImplRetrieveRootFromGrammarFormUsesElasticMock(t *testing.T) {
	service := newTestAggregatorService(t, rootWordHit)

	response, err := service.RetrieveRootFromGrammarForm(context.Background(), &v1.AggregatorRequest{RootWord: "λόγον"})

	assert.Nil(t, err)
	assert.Equal(t, "λόγον", response.Word)
	assert.Equal(t, "λογον", response.UnaccentedWord)
	assert.Equal(t, "λόγος", response.RootWord)
	assert.Equal(t, "noun", response.PartOfSpeech)
	assert.Equal(t, "noun - sing - masc - acc", response.Rule)
	assert.ElementsMatch(t, []string{"word", "account"}, response.Translation)
	assert.ElementsMatch(t, []string{"λόγος"}, response.Variants)
}

func TestAggregatorServiceImplCreateOrUpdateCreatesNewWord(t *testing.T) {
	service := newTestAggregatorService(t, emptyHit, createDocument)
	request := &v1.AggregatorCreationRequest{
		RootWord:     "λέγω",
		Word:         "λέγω",
		Rule:         "1st sing - pres - ind - act",
		Translation:  "I say",
		PartOfSpeech: v1.PartOfSpeech_VERB,
	}

	assert.NotPanics(t, func() {
		service.createOrUpdate(context.Background(), request)
	})
}

func TestAggregatorServiceImplCreateOrUpdateUpdatesExistingWord(t *testing.T) {
	service := newTestAggregatorService(t, rootWordHit, updateDocument)
	request := &v1.AggregatorCreationRequest{
		RootWord:     "λόγος",
		Word:         "λόγῳ",
		Rule:         "noun - sing - masc - dat",
		Translation:  "word",
		PartOfSpeech: v1.PartOfSpeech_NOUN,
	}

	assert.NotPanics(t, func() {
		service.createOrUpdate(context.Background(), request)
	})
}

func newTestAggregatorService(t *testing.T, fixtures ...string) *AggregatorServiceImpl {
	t.Helper()

	rawFixtures := make([][]byte, 0, len(fixtures))
	for _, fixture := range fixtures {
		rawFixtures = append(rawFixtures, []byte(fixture))
	}

	mockElasticClient, err := elastic.NewMockClient(rawFixtures, 200)
	assert.Nil(t, err)

	return &AggregatorServiceImpl{
		Elastic: mockElasticClient,
		Index:   testScholarIndex,
	}
}
