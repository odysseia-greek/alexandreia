package library

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	elastic "github.com/odysseia-greek/agora/aristoteles"
	v1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	"github.com/stretchr/testify/assert"
)

const (
	testLibraryIndex = "eratosthenes-test"
	lemmaHit         = `{
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
				"value": 2,
				"relation": "eq"
			},
			"max_score": 1,
			"hits": [
				{
					"_index": "eratosthenes-test",
					"_type": "_doc",
					"_id": "lemma-1",
					"_score": 1,
					"_source": {
						"greek": "λόγος, -ου, ὁ",
						"english": "word",
						"normalized": "λογος",
						"definitions": [
							{
								"grade": 3,
								"meanings": [
									{
										"language": "en",
										"definition": "word, account"
									}
								]
							}
						]
					}
				},
				{
					"_index": "eratosthenes-test",
					"_type": "_doc",
					"_id": "lemma-2",
					"_score": 1,
					"_source": {
						"greek": "λογίζομαι",
						"english": "reckon",
						"normalized": "λογιζομαι"
					}
				}
			]
		}
	}`
	emptyLemmaHit = `{
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

type fakeCache struct {
	items map[string][]byte
	set   map[string]string
}

func (f *fakeCache) Close() error {
	return nil
}

func (f *fakeCache) Set(key, value string) error {
	if f.set == nil {
		f.set = map[string]string{}
	}
	f.set[key] = value
	return nil
}

func (f *fakeCache) SetBytes(key string, value []byte) error {
	return f.Set(key, string(value))
}

func (f *fakeCache) SetWithTTL(key, value string, ttl time.Duration) error {
	return f.Set(key, value)
}

func (f *fakeCache) SetBytesWithTTL(key string, value []byte, ttl time.Duration) error {
	return f.SetBytes(key, value)
}

func (f *fakeCache) Read(key string) ([]byte, error) {
	if f.items == nil {
		return nil, nil
	}
	return f.items[key], nil
}

func (f *fakeCache) Get(key string) ([]byte, error) { return f.Read(key) }

func (f *fakeCache) GetString(key string) (string, error) {
	value, err := f.Read(key)
	return string(value), err
}

func (f *fakeCache) Exists(key string) (bool, error) {
	value, err := f.Read(key)
	return value != nil, err
}

func (f *fakeCache) Delete(key string) error {
	delete(f.items, key)
	delete(f.set, key)
	return nil
}

func (f *fakeCache) DeletePrefix(prefix string) error {
	for key := range f.items {
		if strings.HasPrefix(key, prefix) {
			delete(f.items, key)
		}
	}
	for key := range f.set {
		if strings.HasPrefix(key, prefix) {
			delete(f.set, key)
		}
	}
	return nil
}

func (f *fakeCache) Stats() (archytas.Stats, error) { return archytas.Stats{}, nil }

func TestResolveReturnsCachedCandidates(t *testing.T) {
	cachedCandidates := []*v1.Candidate{
		{Lemma: "λόγος", Score: 100, Levenshtein: 0, Glosses: []string{"word"}},
	}
	cached, err := json.Marshal(cachedCandidates)
	assert.Nil(t, err)

	service := LibraryServiceImpl{
		Cache: &fakeCache{items: map[string][]byte{resolveCacheKey("λόγος"): cached}},
	}

	response, err := service.Resolve(context.Background(), &v1.ResolveRequest{Term: "λόγος", Limit: 5})

	assert.Nil(t, err)
	assert.Equal(t, "λόγος", response.Term)
	assert.Equal(t, "λογος", response.NormalizedTerm)
	assert.Len(t, response.Candidates, 1)
	assert.Equal(t, "λόγος", response.Candidates[0].Lemma)
	assert.Equal(t, int32(100), response.Candidates[0].Score)
}

func TestResolveQueriesElasticAndCachesCandidates(t *testing.T) {
	cache := &fakeCache{}
	service := newTestLibraryService(t, cache, lemmaHit)

	response, err := service.Resolve(context.Background(), &v1.ResolveRequest{Term: "λόγος", Limit: 5})

	assert.Nil(t, err)
	assert.Equal(t, "λόγος", response.Term)
	assert.Equal(t, "λογος", response.NormalizedTerm)
	assert.Len(t, response.Candidates, 1)
	assert.Equal(t, "λόγος", response.Candidates[0].Lemma)
	assert.Equal(t, int32(100), response.Candidates[0].Score)
	assert.ElementsMatch(t, []string{"word, account"}, response.Candidates[0].Glosses)
	assert.NotEmpty(t, cache.set[resolveCacheKey("λόγος")])
}

func TestResolveFallsBackToNormalizedQuery(t *testing.T) {
	cache := &fakeCache{}
	service := newTestLibraryService(t, cache, emptyLemmaHit, lemmaHit)

	response, err := service.Resolve(context.Background(), &v1.ResolveRequest{Term: "λόγος", Limit: 5})

	assert.Nil(t, err)
	assert.Len(t, response.Candidates, 1)
	assert.Equal(t, "λόγος", response.Candidates[0].Lemma)
	assert.Equal(t, int32(90), response.Candidates[0].Score)
}

func TestQueryElasticReturnsHitsTotal(t *testing.T) {
	service := newTestLibraryService(t, &fakeCache{}, lemmaHit)

	response, total, err := service.queryElastic(context.Background(), "λόγος", false, 10)

	assert.Nil(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, response.Hits.Hits, 2)
}

func TestReviewCandidatesSortsByScoreThenLemma(t *testing.T) {
	service := LibraryServiceImpl{}
	lemmas := []Lemma{
		{Greek: "βίος", English: "life"},
		{Greek: "λόγος, -ου, ὁ", English: "word"},
		{Greek: "λόγον", English: "word in accusative"},
	}

	candidates, err := service.reviewCandidates(lemmas, "λόγος", false)

	assert.Nil(t, err)
	assert.Len(t, candidates, 3)
	assert.Equal(t, "λόγος", candidates[0].Lemma)
	assert.Equal(t, int32(100), candidates[0].Score)
	assert.Equal(t, "λόγον", candidates[1].Lemma)
	assert.Equal(t, int32(80), candidates[1].Score)
}

func TestReviewCandidatesReturnsErrorWithoutUsableLemmas(t *testing.T) {
	service := LibraryServiceImpl{}

	candidates, err := service.reviewCandidates([]Lemma{{Greek: "   "}}, "λόγος", false)

	assert.Nil(t, candidates)
	assert.EqualError(t, err, "no candidates found")
}

func TestReviewCandidatesPrefersStructuredExactLemmaOverLegacyDuplicates(t *testing.T) {
	service := LibraryServiceImpl{}
	lemmas := []Lemma{
		{Greek: "γίγνομαι", English: "become, be born"},
		{
			Greek:        "γίγνομαι",
			Normalized:   "γιγνομαι",
			PartOfSpeech: "verb",
			Verb:         &VerbInfo{PrincipalParts: []string{"γίγνομαι", "γενήσομαι", "ἐγενόμην"}},
			Definitions: []*Definition{{
				Grade:    3,
				Meanings: []*Meaning{{Language: "en", Definition: "to become; to be born"}},
			}},
		},
		{Greek: "γίγνομαι", English: "to become, to be"},
		{Greek: "ἐκγίγνομαι", English: "to be born of"},
	}

	candidates, err := service.reviewCandidates(lemmas, "γίγνομαι", false)

	assert.NoError(t, err)
	assert.Len(t, candidates, 1)
	assert.Equal(t, "γίγνομαι", candidates[0].Lemma)
	assert.Equal(t, int32(100), candidates[0].Score)
	assert.Equal(t, []string{"to become; to be born"}, candidates[0].Glosses)
}

func TestReviewCandidatesRetainsFuzzyResultsWithoutCanonicalEntry(t *testing.T) {
	service := LibraryServiceImpl{}
	lemmas := []Lemma{
		{Greek: "λόγον", English: "word in accusative"},
		{Greek: "λογίζομαι", English: "reckon"},
	}

	candidates, err := service.reviewCandidates(lemmas, "λόγος", false)

	assert.NoError(t, err)
	assert.Len(t, candidates, 2)
}

func TestExtractGlossesPrefersGradeThreeEnglishDefinitions(t *testing.T) {
	lemma := Lemma{
		English: "fallback",
		Definitions: []*Definition{
			{
				Grade: 2,
				Meanings: []*Meaning{
					{Language: "en", Definition: "ignored"},
				},
			},
			{
				Grade: 3,
				Meanings: []*Meaning{
					{Language: "nl", Definition: "genegeerd"},
					{Language: "en", Definition: "preferred"},
				},
			},
		},
	}

	assert.ElementsMatch(t, []string{"preferred"}, extractGlosses(lemma))
}

func TestExtractGlossesFallsBackToEnglish(t *testing.T) {
	assert.ElementsMatch(t, []string{"market"}, extractGlosses(Lemma{English: "market"}))
	assert.Nil(t, extractGlosses(Lemma{}))
}

func TestCalculateScore(t *testing.T) {
	service := LibraryServiceImpl{}

	assert.Equal(t, 100, service.calculateScore(0, false))
	assert.Equal(t, 90, service.calculateScore(0, true))
	assert.Equal(t, 80, service.calculateScore(1, false))
	assert.Equal(t, 60, service.calculateScore(2, false))
	assert.Equal(t, 10, service.calculateScore(3, false))
}

func TestLevenshteinDistance(t *testing.T) {
	assert.Equal(t, 0, levenshteinDistance("λόγος", "λόγος"))
	assert.Equal(t, 1, levenshteinDistance("λόγος", "λόγον"))
	assert.Equal(t, 3, levenshteinDistance("kitten", "sitting"))
}

func TestExtractBaseWord(t *testing.T) {
	assert.Equal(t, "λόγος", extractBaseWord("λόγος, -ου, ὁ"))
	assert.Equal(t, "ἀγορά", extractBaseWord("η ἀγορά"))
	assert.Equal(t, "βίος", extractBaseWord("βίος."))
}

func newTestLibraryService(t *testing.T, cache *fakeCache, fixtures ...string) *LibraryServiceImpl {
	t.Helper()

	rawFixtures := make([][]byte, 0, len(fixtures))
	for _, fixture := range fixtures {
		rawFixtures = append(rawFixtures, []byte(fixture))
	}

	mockElasticClient, err := elastic.NewMockClient(rawFixtures, 200)
	assert.Nil(t, err)

	return &LibraryServiceImpl{
		Elastic: mockElasticClient,
		Index:   testLibraryIndex,
		Cache:   cache,
	}
}
