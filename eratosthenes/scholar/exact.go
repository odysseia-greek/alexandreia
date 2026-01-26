package scholar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/odysseia-greek/agora/aristoteles/models"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/transform"
	v1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (e *ExactServiceImpl) Health(ctx context.Context, request *emptypb.Empty) (*v1.HealthResponse, error) {
	elasticHealth := e.Elastic.Health().Info()
	dbHealth := &koinos.DatabaseHealth{
		Healthy:       elasticHealth.Healthy,
		ClusterName:   elasticHealth.ClusterName,
		ServerName:    elasticHealth.ServerName,
		ServerVersion: elasticHealth.ServerVersion,
	}

	return &koinos.HealthResponse{
		Healthy:        true,
		Time:           time.Now().String(),
		DatabaseHealth: dbHealth,
		Version:        e.Version,
	}, nil
}

func (e *ExactServiceImpl) queryElastic(ctx context.Context, word, language string, normalized bool, results int32) (*models.Response, int64, error) {
	var query map[string]interface{}

	if normalized {
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"match": map[string]interface{}{
					"normalized": word,
				},
			},
		}
	} else {
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"should": []interface{}{
						map[string]interface{}{
							"prefix": map[string]interface{}{
								fmt.Sprintf("%s.keyword", language): fmt.Sprintf("%s,", word),
							},
						},
						map[string]interface{}{
							"term": map[string]interface{}{
								fmt.Sprintf("%s.keyword", language): word,
							},
						},
					},
				},
			},
			"size": results,
		}
	}

	elasticResponse, err := e.Elastic.Query().Match(e.Index, query)
	if err != nil {
		return nil, 0, fmt.Errorf("error querying elastic: %w", err)
	}

	hitsTotal := int64(0)
	if elasticResponse.Hits.Hits != nil {
		hitsTotal = elasticResponse.Hits.Total.Value
	}
	go comedy.DatabaseSpan(query, hitsTotal, elasticResponse.Took, ctx, e.Streamer)

	return elasticResponse, hitsTotal, nil
}

func extractBaseWord(queryWord string) (string, string) {
	splitWord := strings.Split(queryWord, " ")

	// Known Greek pronouns
	greekPronouns := map[string]bool{"η": true, "ο": true, "το": true}

	// Function to clean punctuation from a word
	cleanWord := func(word string) string {
		return strings.Trim(word, ",.!?-") // Add any other punctuation as needed
	}

	// Iterate through the words
	for _, word := range splitWord {
		cleanedWord := cleanWord(word)

		if strings.HasPrefix(cleanedWord, "-") {
			// Skip words starting with "-"
			continue
		}

		if _, isPronoun := greekPronouns[cleanedWord]; !isPronoun {
			// If the word is not a pronoun, it's likely the correct word
			// Normalize and split the input
			strippedWord := transform.RemoveAccents(strings.ToLower(cleanedWord))
			return cleanedWord, strippedWord
		}
	}

	return queryWord, transform.RemoveAccents(strings.ToLower(queryWord))
}

func (e *ExactServiceImpl) recordRequest(ctx context.Context) {
	e.totalRequests.Add(1)

	if p, ok := peer.FromContext(ctx); ok {
		ip := p.Addr.String()
		val, _ := e.ipMap.LoadOrStore(ip, new(uint64))
		atomic.AddUint64(val.(*uint64), 1)
	}
}

func (e *ExactServiceImpl) StartReporting(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				total := e.totalRequests.Load()
				if total == 0 {
					continue
				}

				stats := fmt.Sprintf("Service Stats (Last Minute) - Total Requests: %d\nIP Breakdown:", total)
				e.ipMap.Range(func(key, value any) bool {
					count := atomic.LoadUint64(value.(*uint64))
					stats += fmt.Sprintf("\n  - %s: %d", key.(string), count)
					return true
				})
				logging.Info(stats)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
