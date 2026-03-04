package library

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/odysseia-greek/agora/aristoteles/models"
	"github.com/odysseia-greek/agora/plato/logging"
	ariv1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	v1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
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

func (l *LibraryServiceImpl) Analyze(ctx context.Context, request *v1.AnalyzeRequest) (*v1.AnalyzeResponse, error) {
	start := time.Now()
	rootword := strings.TrimSpace(request.GetRootword())
	if rootword == "" {
		return nil, fmt.Errorf("rootword is required")
	}
	limit := sanitizeLimit(request.GetLimit())

	response := &v1.AnalyzeResponse{
		Rootword: rootword,
		DirectResult: &v1.DirectResult{
			RequestedWord: rootword,
		},
	}

	entry, words, conjugations := l.resolveForms(ctx, rootword)
	if entry != nil {
		response.PartOfSpeech = entry.PartOfSpeech.String()
	}
	response.Conjugations = conjugations

	directQuery := createGreekTextQuery([]string{rootword}, limit)
	directResponse, directHits, err := l.queryTexts(ctx, directQuery)
	if err != nil {
		return nil, err
	}

	directResults, err := collectAnalyzeResults(directResponse, []string{rootword}, limit)
	if err != nil {
		return nil, err
	}
	response.DirectResult.Texts = directResults

	expandedWords := expandedWordsWithoutRoot(words, rootword)
	if len(expandedWords) == 0 {
		if len(response.DirectResult.Texts) == 0 {
			return nil, fmt.Errorf("no hits for rootword %q", rootword)
		}

		logging.Debug(fmt.Sprintf(
			"analyze done rootword=%s words=%d directHits=%d directResults=%d expandedResults=%d limit=%d duration=%s",
			rootword, len(words), directHits, len(response.DirectResult.Texts), len(response.Results), limit, time.Since(start),
		))
		return response, nil
	}

	expandedQuery := createGreekTextQuery(expandedWords, limit)
	elasticResponse, expandedHits, err := l.queryTexts(ctx, expandedQuery)
	if err != nil {
		return nil, err
	}

	if len(response.DirectResult.Texts) == 0 && len(elasticResponse.Hits.Hits) == 0 {
		return nil, fmt.Errorf("no hits for rootword %q", rootword)
	}

	results, err := collectAnalyzeResults(elasticResponse, expandedWords, limit)
	if err != nil {
		return nil, err
	}

	response.Results = results

	logging.Debug(fmt.Sprintf(
		"analyze done rootword=%s words=%d directHits=%d expandedHits=%d directResults=%d expandedResults=%d limit=%d duration=%s",
		rootword, len(words), directHits, expandedHits, len(response.DirectResult.Texts), len(response.Results), limit, time.Since(start),
	))

	return response, nil
}

func collectAnalyzeResults(response *models.Response, words []string, limit uint32) ([]*v1.AnalyzeResult, error) {
	var results []*v1.AnalyzeResult

	if response == nil {
		return results, nil
	}

	for _, hit := range response.Hits.Hits {
		text, err := decodeTextDocument(hit.Source)
		if err != nil {
			return nil, err
		}

		results = append(results, analyzeTextSections(text, words)...)
		if uint32(len(results)) >= limit {
			results = results[:limit]
			break
		}
	}

	return results, nil
}

func sanitizeLimit(limit uint32) uint32 {
	const (
		defaultLimit uint32 = 5
		maxLimit     uint32 = 20
	)

	if limit == 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}

	return limit
}

func (l *LibraryServiceImpl) resolveForms(ctx context.Context, rootword string) (*ariv1.RootWordResponse, []string, []*v1.Conjugation) {
	entry, err := l.Aggregator.RetrieveEntry(ctx, &ariv1.AggregatorRequest{RootWord: rootword})
	if err != nil {
		logging.Error(fmt.Sprintf("failed to retrieve entry for %s: %s", rootword, err.Error()))
		return nil, []string{rootword}, nil
	}

	var rootWordFound bool
	words := make([]string, 0)
	wordSet := make(map[string]struct{})
	conjugations := make([]*v1.Conjugation, 0)

	for _, category := range entry.Categories {
		for _, form := range category.Forms {
			if form.Word == entry.RootWord {
				rootWordFound = true
			}

			if _, exists := wordSet[form.Word]; !exists {
				wordSet[form.Word] = struct{}{}
				words = append(words, form.Word)
			}

			conjugations = append(conjugations, &v1.Conjugation{
				Word: form.Word,
				Rule: form.Rule,
			})
		}
	}

	if !rootWordFound {
		if _, exists := wordSet[rootword]; !exists {
			words = append(words, rootword)
		}
	}

	if len(words) == 0 {
		words = append(words, rootword)
	}

	return entry, words, conjugations
}

func (l *LibraryServiceImpl) queryTexts(ctx context.Context, query map[string]interface{}) (*models.Response, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	response, err := l.Elastic.Query().MatchWithContext(ctx, l.Index, query)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying elastic: %w", err)
	}

	hits := int64(0)
	if response != nil && response.Hits.Hits != nil {
		hits = response.Hits.Total.Value
		if l.Streamer != nil {
			go comedy.DatabaseSpan(query, hits, response.Took, ctx, l.Streamer)
		}
	}

	return response, hits, nil
}

func decodeTextDocument(source interface{}) (TextDocument, error) {
	var text TextDocument

	raw, err := json.Marshal(source)
	if err != nil {
		return text, fmt.Errorf("marshal hit source: %w", err)
	}

	if err := json.Unmarshal(raw, &text); err != nil {
		return text, fmt.Errorf("unmarshal text document: %w", err)
	}

	return text, nil
}

func analyzeTextSections(text TextDocument, words []string) []*v1.AnalyzeResult {
	results := make([]*v1.AnalyzeResult, 0)

	for _, section := range text.Rhemai {
		highlighted, found := highlightSection(section, words)
		if !found {
			continue
		}

		results = append(results, &v1.AnalyzeResult{
			ReferenceLink: buildReferenceLink(text.Author, text.Book, text.Reference),
			Author:        text.Author,
			Book:          text.Book,
			Reference:     text.Reference,
			Text: &v1.Rhema{
				Greek:        highlighted.Greek,
				Translations: highlighted.Translations,
				Section:      highlighted.Section,
			},
		})
	}

	return results
}

func highlightSection(section Rhema, words []string) (Rhema, bool) {
	deduped := uniqueWords(words)
	sort.SliceStable(deduped, func(i, j int) bool {
		return len([]rune(deduped[i])) > len([]rune(deduped[j]))
	})

	found := false
	for _, word := range deduped {
		if word == "" {
			continue
		}
		if strings.Contains(section.Greek, word) {
			found = true
			section.Greek = strings.ReplaceAll(section.Greek, word, fmt.Sprintf("&&&%s&&&", word))
		}
	}

	return section, found
}

func uniqueWords(words []string) []string {
	seen := make(map[string]struct{}, len(words))
	out := make([]string, 0, len(words))

	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if _, exists := seen[word]; exists {
			continue
		}
		seen[word] = struct{}{}
		out = append(out, word)
	}

	return out
}

func expandedWordsWithoutRoot(words []string, rootword string) []string {
	filtered := make([]string, 0, len(words))

	for _, word := range uniqueWords(words) {
		if word == rootword {
			continue
		}
		filtered = append(filtered, word)
	}

	return filtered
}

func buildReferenceLink(author, book, reference string) string {
	return fmt.Sprintf(
		"/texts?author=%s&book=%s&reference=%s",
		url.QueryEscape(author),
		url.QueryEscape(book),
		url.QueryEscape(reference),
	)
}

func createGreekTextQuery(words []string, limit uint32) map[string]interface{} {
	shouldClauses := make([]map[string]interface{}, 0, len(words))

	for _, word := range uniqueWords(words) {
		shouldClauses = append(shouldClauses, map[string]interface{}{
			"nested": map[string]interface{}{
				"path": "rhemai",
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"should": []map[string]interface{}{
							{
								"match": map[string]interface{}{
									"rhemai.greek": word,
								},
							},
						},
					},
				},
			},
		})
	}

	return map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": shouldClauses,
			},
		},
	}
}
