package library

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/odysseia-greek/agora/aristoteles/models"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/transform"
	v1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (l *LibraryServiceImpl) Health(ctx context.Context, request *emptypb.Empty) (*v1.HealthResponse, error) {
	elasticHealth := l.Elastic.Health().Info()
	dbHealth := &v1.DatabaseHealth{
		Healthy:       elasticHealth.Healthy,
		ClusterName:   elasticHealth.ClusterName,
		ServerName:    elasticHealth.ServerName,
		ServerVersion: elasticHealth.ServerVersion,
	}

	return &v1.HealthResponse{
		Healthy:        true,
		Time:           time.Now().String(),
		DatabaseHealth: dbHealth,
		Version:        l.Version,
	}, nil
}

func (l *LibraryServiceImpl) Resolve(ctx context.Context, entry *v1.ResolveRequest) (*v1.ResolveResponse, error) {
	normalizedWord := transform.RemoveAccents(entry.Term)
	response := &v1.ResolveResponse{
		Term:           entry.Term,
		NormalizedTerm: normalizedWord,
		Candidates:     nil,
	}

	normalized := false
	elasticResponse, _, err := l.queryElastic(ctx, entry.Term, normalized, int32(entry.Limit))
	if err != nil {
		return nil, err
	}

	if len(elasticResponse.Hits.Hits) == 0 {
		logging.Debug("no hits found trying with a word without diacretics")
		normalized = true
		elasticResponse, _, err = l.queryElastic(ctx, normalizedWord, normalized, int32(entry.Limit))
		if err != nil {
			return nil, err
		}
	}

	var lemmas []Lemma
	for _, hit := range elasticResponse.Hits.Hits {
		source, _ := json.Marshal(hit.Source)
		var src Lemma
		if err := json.Unmarshal(source, &src); err != nil {
			b, _ := json.Marshal(hit.Source)
			if err2 := json.Unmarshal(b, &src); err2 != nil {
				return nil, fmt.Errorf("decode _source: %w", err2)
			}
		}

		lemmas = append(lemmas, src)
	}

	candidates, err := l.reviewCandidates(lemmas, entry.Term, normalized)
	if err != nil {
		logging.Error(err.Error())
	}

	response.Candidates = candidates

	return response, nil
}

func (l *LibraryServiceImpl) queryElastic(ctx context.Context, word string, normalized bool, results int32) (*models.Response, int64, error) {
	var query map[string]interface{}

	keyword := "greek.keyword"

	if normalized {
		keyword = "normalized.keyword"
	}

	query = map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{
						"prefix": map[string]interface{}{
							keyword: fmt.Sprintf("%s,", word),
						},
					},
					map[string]interface{}{
						"term": map[string]interface{}{
							keyword: word,
						},
					},
				},
			},
		},
		"size": results,
	}

	elasticResponse, err := l.Elastic.Query().Match(l.Index, query)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying elastic: %w", err)
	}

	hitsTotal := int64(0)
	if elasticResponse.Hits.Hits != nil {
		hitsTotal = elasticResponse.Hits.Total.Value
	}
	go comedy.DatabaseSpan(query, hitsTotal, elasticResponse.Took, ctx, l.Streamer)

	return elasticResponse, hitsTotal, nil
}

func (l *LibraryServiceImpl) reviewCandidates(lemmas []Lemma, queryTerm string, normalized bool) ([]*v1.Candidate, error) {
	var candidates []*v1.Candidate

	for _, lemma := range lemmas {
		lemmaGreek := strings.TrimSpace(lemma.Greek)
		if lemmaGreek == "" {
			continue
		}

		baseWord := extractBaseWord(lemmaGreek)

		// compute distance on normalized forms
		d := levenshteinDistance(queryTerm, baseWord)
		score := l.calculateScore(int32(d), normalized)

		glosses := extractGlosses(lemma)

		c := &v1.Candidate{
			Lemma:       baseWord,
			Score:       int32(score),
			Levenshtein: int32(d),
			Glosses:     glosses,
		}

		candidates = append(candidates, c)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidates found")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Lemma < candidates[j].Lemma
	})

	return candidates, nil
}

func extractGlosses(l Lemma) []string {
	for _, def := range l.Definitions {
		if def.Grade == 3 {
			var gs []string
			for _, m := range def.Meanings {
				if m.Language == "en" && m.Definition != "" {
					gs = append(gs, m.Definition)
				}
			}
			if len(gs) > 0 {
				return gs
			}
		}
	}
	if l.English != "" {
		return []string{l.English}
	}
	return nil
}

func (l *LibraryServiceImpl) calculateScore(d int32, normalized bool) int {
	baseScore := 100

	if normalized {
		baseScore -= 10
	}

	if d == 0 {
		return baseScore
	} else if d == 1 {
		return baseScore - 20
	} else if d == 2 {
		return baseScore - 40
	}

	return 10
}

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)

	alen := len(ar)
	blen := len(br)

	column := make([]int, alen+1)
	for y := 1; y <= alen; y++ {
		column[y] = y
	}

	for x := 1; x <= blen; x++ {
		column[0] = x
		lastKey := x - 1
		for y := 1; y <= alen; y++ {
			oldKey := column[y]
			incr := 0
			if ar[y-1] != br[x-1] {
				incr = 1
			}
			column[y] = minimum(column[y]+1, column[y-1]+1, lastKey+incr)
			lastKey = oldKey
		}
	}

	return column[alen]
}

func minimum(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
	} else {
		if b < c {
			return b
		}
	}
	return c
}

func extractBaseWord(queryWord string) string {
	splitWord := strings.Split(queryWord, " ")

	// Known Greek pronouns
	greekPronouns := map[string]bool{"η": true, "ο": true, "το": true}

	// Function to clean punctuation from a word
	cleanWord := func(word string) string {
		return strings.Trim(word, ",.!?-")
	}

	// Iterate through the words
	for _, word := range splitWord {
		cleanedWord := cleanWord(word)

		if strings.HasPrefix(cleanedWord, "-") {
			// Skip words starting with "-"
			continue
		}

		if _, isPronoun := greekPronouns[cleanedWord]; !isPronoun {
			return cleanedWord
		}
	}

	return queryWord
}
