package scholar

import (
	"context"
	"testing"

	elastic "github.com/odysseia-greek/agora/aristoteles"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/stretchr/testify/assert"
)

const createIndexResponse = `{
	"acknowledged": true,
	"shards_acknowledged": true,
	"index": "aristarchos-test"
}`

func TestAggregatorServiceImplHealth(t *testing.T) {
	service := AggregatorServiceImpl{}

	response, err := service.Health(context.Background(), &v1.HealthRequest{})

	assert.Nil(t, err)
	assert.True(t, response.Health)
}

func TestMapCategoryToEnum(t *testing.T) {
	tests := []struct {
		name     string
		category string
		expected v1.PartOfSpeech
	}{
		{name: "verb", category: "verb", expected: v1.PartOfSpeech_VERB},
		{name: "noun", category: "noun", expected: v1.PartOfSpeech_NOUN},
		{name: "participle", category: "participle", expected: v1.PartOfSpeech_PARTICIPLE},
		{name: "preposition", category: "preposition", expected: v1.PartOfSpeech_PREPOSITION},
		{name: "adverb", category: "adverb", expected: v1.PartOfSpeech_ADVERB},
		{name: "article", category: "article", expected: v1.PartOfSpeech_ARTICLE},
		{name: "conjunction", category: "conjunction", expected: v1.PartOfSpeech_CONJUNCTION},
		{name: "pronoun", category: "pronoun", expected: v1.PartOfSpeech_PRONOUN},
		{name: "particle", category: "particle", expected: v1.PartOfSpeech_PARTICLE},
		{name: "unknown", category: "interjection", expected: v1.PartOfSpeech_UNKNOWN_CATEGORY},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapCategoryToEnum(tt.category))
		})
	}
}

func TestMapEnumToCategory(t *testing.T) {
	tests := []struct {
		name     string
		category v1.PartOfSpeech
		expected string
	}{
		{name: "verb", category: v1.PartOfSpeech_VERB, expected: "verb"},
		{name: "noun", category: v1.PartOfSpeech_NOUN, expected: "noun"},
		{name: "participle", category: v1.PartOfSpeech_PARTICIPLE, expected: "participle"},
		{name: "preposition", category: v1.PartOfSpeech_PREPOSITION, expected: "preposition"},
		{name: "adverb", category: v1.PartOfSpeech_ADVERB, expected: "adverb"},
		{name: "article", category: v1.PartOfSpeech_ARTICLE, expected: "article"},
		{name: "conjunction", category: v1.PartOfSpeech_CONJUNCTION, expected: "conjunction"},
		{name: "pronoun", category: v1.PartOfSpeech_PRONOUN, expected: "pronoun"},
		{name: "particle", category: v1.PartOfSpeech_PARTICLE, expected: "particle"},
		{name: "unknown", category: v1.PartOfSpeech_UNKNOWN_CATEGORY, expected: "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapEnumToCategory(tt.category))
		})
	}
}

func TestMapAndHandleGrammaticalCategories(t *testing.T) {
	service := AggregatorServiceImpl{}
	request := &v1.AggregatorCreationRequest{
		RootWord:     "λόγος",
		Word:         "λόγον",
		Rule:         "noun - sing - masc - acc",
		Translation:  "word",
		PartOfSpeech: v1.PartOfSpeech_NOUN,
	}

	entry, err := service.mapAndHandleGrammaticalCategories(request)

	assert.Nil(t, err)
	assert.Equal(t, "λόγος", entry.RootWord)
	assert.Equal(t, "λογος", entry.UnaccentedWord)
	assert.Equal(t, "noun", entry.PartOfSpeech)
	assert.ElementsMatch(t, []string{"word"}, entry.Translations)
	assert.Len(t, entry.Categories, 1)
	assert.Len(t, entry.Categories[0].Forms, 1)
	assert.Equal(t, "λόγον", entry.Categories[0].Forms[0].Word)
	assert.Equal(t, "noun - sing - masc - acc", entry.Categories[0].Forms[0].Rule)
}

func TestUnmarshalRootWordEntry(t *testing.T) {
	entry, err := UnmarshalRootWordEntry([]byte(`{
		"rootWord": "λέγω",
		"partOfSpeech": "verb",
		"translations": ["I say"],
		"unaccented": "λεγω",
		"variants": [
			{
				"searchTerm": "λέγω",
				"score": 2
			}
		],
		"categories": [
			{
				"forms": [
					{
						"word": "λέγω",
						"rule": "1st sing - pres - ind - act"
					}
				]
			}
		]
	}`))

	assert.Nil(t, err)
	assert.Equal(t, "λέγω", entry.RootWord)
	assert.Equal(t, "verb", entry.PartOfSpeech)
	assert.ElementsMatch(t, []string{"I say"}, entry.Translations)
	assert.Equal(t, "λεγω", entry.UnaccentedWord)
	assert.Equal(t, "λέγω", entry.Variants[0].SearchTerm)
	assert.Equal(t, 2, entry.Variants[0].Score)
	assert.Equal(t, "λέγω", entry.Categories[0].Forms[0].Word)
}

func TestUnmarshalRootWordEntryMalformedJSON(t *testing.T) {
	entry, err := UnmarshalRootWordEntry([]byte(`{"rootWord":`))

	assert.Error(t, err)
	assert.Empty(t, entry.RootWord)
}

func TestCreateScholarIndexMapping(t *testing.T) {
	mapping := createScholarIndexMapping("hot-plain")

	settings := mapping["settings"].(map[string]interface{})
	indexSettings := settings["index"].(map[string]interface{})
	assert.Equal(t, "hot-plain", indexSettings["lifecycle.name"])
	assert.Equal(t, "30s", indexSettings["refresh_interval"])

	mappings := mapping["mappings"].(map[string]interface{})
	properties := mappings["properties"].(map[string]interface{})
	assert.Equal(t, "text", properties["rootWord"].(map[string]interface{})["type"])
	assert.Equal(t, "text", properties["unaccented"].(map[string]interface{})["type"])
	assert.Equal(t, "keyword", properties["partOfSpeech"].(map[string]interface{})["type"])
	assert.Equal(t, "nested", properties["variants"].(map[string]interface{})["type"])
	assert.Equal(t, "nested", properties["categories"].(map[string]interface{})["type"])
}

func TestCreateIndexAtStartupUsesElasticMock(t *testing.T) {
	mockElasticClient, err := elastic.NewMockClient([][]byte{[]byte(createIndexResponse)}, 200)
	assert.Nil(t, err)

	err = CreateIndexAtStartup("hot-plain", testScholarIndex, mockElasticClient)

	assert.Nil(t, err)
}

func TestCreateIndexAtStartupIgnoresExistingIndexError(t *testing.T) {
	mockElasticClient, err := elastic.NewMockClient([][]byte{[]byte(`{
		"error": {
			"type": "resource_already_exists_exception",
			"reason": "index [aristarchos-test] already exists"
		},
		"status": 400
	}`)}, 500)
	assert.Nil(t, err)

	err = CreateIndexAtStartup("hot-plain", testScholarIndex, mockElasticClient)

	assert.Nil(t, err)
}
