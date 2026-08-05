package grammar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/agora/plato/service"
	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	sv1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func (d *DionysosHandler) Health(ctx context.Context, request *v1.HealthRequest) (*v1.HealthResponse, error) {
	return d.detailedHealth(ctx), nil
}

func (d *DionysosHandler) CheckGrammar(ctx context.Context, request *v1.CheckGrammarRequest) (*v1.CheckGrammarResponse, error) {
	word := strings.TrimSpace(request.GetWord())
	if word == "" {
		return nil, status.Error(codes.InvalidArgument, "word is required")
	}

	requestID := requestIDFromContext(ctx)
	auditLog := newGrammarAuditLog(requestID, word)
	auditLog.Add(GrammarAuditEvent{
		Step:   "request.received",
		Status: "ok",
		Reason: "grammar check request accepted",
	})

	results, err := d.checkGrammarResults(ctx, word, requestID, auditLog)
	if err != nil {
		auditLog.Emit()
		return nil, err
	}

	auditLog.Emit()
	response := &v1.CheckGrammarResponse{
		Results: mapDeclensionResults(results.Results),
	}
	if request.GetIncludeAudit() {
		response.Audit = mapGrammarAudit(auditLog)
	}

	return response, nil
}

func (d *DionysosHandler) Research(ctx context.Context, request *v1.ResearchRequest) (*v1.ResearchResponse, error) {
	rootword := strings.TrimSpace(request.GetRootword())
	if rootword == "" {
		return nil, status.Error(codes.InvalidArgument, "rootword is required")
	}
	if d.ScholarService == nil || d.ScholarService.Client == nil {
		return nil, status.Error(codes.FailedPrecondition, "scholar service is not configured")
	}

	limit := request.GetLimit()
	if limit == 0 {
		limit = 5
	}

	outCtx, cancel := d.outgoingCtx(ctx)
	defer cancel()

	scholarRequest := &sv1.AnalyzeRequest{
		Rootword: rootword,
		Limit:    limit,
	}
	results, err := d.ScholarService.Client.Analyze(outCtx, scholarRequest)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "research failed: %v", err)
	}
	mappedResults := limitResearchResults(mapResearchResults(results), limit)

	return &v1.ResearchResponse{
		Rootword:     results.Rootword,
		PartOfSpeech: results.PartOfSpeech,
		Conjugations: mapResearchConjugations(results.Conjugations),
		Results:      mappedResults,
	}, nil
}

func limitResearchResults(results []*v1.AnalyzeResult, limit uint32) []*v1.AnalyzeResult {
	if uint32(len(results)) <= limit {
		return results
	}
	return results[:limit]
}

func (d *DionysosHandler) checkGrammarResults(ctx context.Context, word, requestID string, auditLog *GrammarAuditLog) (*models.DeclensionTranslationResults, error) {
	if d.Cache == nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.lookup",
			Status: "skipped",
			Reason: "cache client is not configured",
			Source: "cache",
		})
	} else {
		cacheItem, _ := d.Cache.Read(grammarCacheKey(word))
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.lookup",
			Status: "ok",
			Reason: "checked grammar cache",
			Source: "cache",
			Details: []string{
				fmt.Sprintf("hit=%t", cacheItem != nil),
			},
		})
		if cacheItem != nil {
			var cache models.DeclensionTranslationResults
			if err := json.Unmarshal(cacheItem, &cache); err != nil {
				auditLog.Add(GrammarAuditEvent{
					Step:   "cache.unmarshal",
					Status: "failed",
					Reason: err.Error(),
					Source: "cache",
				})
				return nil, status.Errorf(codes.Internal, "cached payload could not be parsed: %v", err)
			}
			auditLog.Add(GrammarAuditEvent{
				Step:        "cache.return",
				Status:      "ok",
				Reason:      "returning cached grammar result",
				Source:      "cache",
				ResultCount: len(cache.Results),
			})
			_ = d.sendWordsToAggregator(ctx, &cache, requestID)
			auditLog.Complete("success", "cache", "cache hit satisfied request")
			return &cache, nil
		}
	}

	declensions, err := d.StartFindingRules(ctx, word, auditLog)
	if err != nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "rule_engine.execute",
			Status: "failed",
			Reason: err.Error(),
			Source: "rule-engine",
		})
		auditLog.Complete("error", "rule-engine", "rule engine execution failed")
		return nil, status.Errorf(codes.Internal, "rule engine execution failed: %v", err)
	}
	if declensions == nil || len(declensions.Results) == 0 {
		auditLog.Add(GrammarAuditEvent{
			Step:   "rule_engine.result",
			Status: "empty",
			Reason: "no declension or dictionary matches were found",
			Source: "rule-engine",
		})
		auditLog.Complete("not_found", "rule-engine", "no options found")
		return nil, status.Error(codes.NotFound, "no options found")
	}

	if err := d.sendWordsToAggregator(ctx, declensions, requestID); err != nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "aggregator.send",
			Status: "failed",
			Reason: err.Error(),
			Source: "aggregator",
		})
		logging.Error(fmt.Sprintf("error in aggregator: %s", err.Error()))
	} else {
		auditLog.Add(GrammarAuditEvent{
			Step:        "aggregator.send",
			Status:      "ok",
			Reason:      "generated results forwarded to aggregator",
			Source:      "aggregator",
			ResultCount: len(declensions.Results),
		})
	}

	d.cacheGrammarResults(word, declensions, auditLog)
	auditLog.Complete("success", "rule-engine", "rule engine generated the final result set")

	return declensions, nil
}

func (d *DionysosHandler) cacheGrammarResults(word string, results *models.DeclensionTranslationResults, auditLog *GrammarAuditLog) {
	if d.Cache == nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.write",
			Status: "skipped",
			Reason: "cache client is not configured",
			Source: "cache",
		})
		return
	}

	stringifiedDeclension, err := json.Marshal(results)
	if err != nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.write",
			Status: "failed",
			Reason: err.Error(),
			Source: "cache",
		})
		return
	}

	ttl := time.Hour
	if err := d.Cache.SetWithTTL(grammarCacheKey(word), string(stringifiedDeclension), ttl); err != nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.write",
			Status: "failed",
			Reason: err.Error(),
			Source: "cache",
		})
		logging.Error(fmt.Sprintf("error setting cache: %s", err.Error()))
		return
	}

	auditLog.Add(GrammarAuditEvent{
		Step:   "cache.write",
		Status: "ok",
		Reason: "stored generated results in cache",
		Source: "cache",
		Details: []string{
			fmt.Sprintf("ttl=%s", ttl),
		},
	})
}

func grammarCacheKey(word string) string {
	return "grammar:v2:" + normalizeGrammarInput(word)
}

func requestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(config.DefaultTracingName).(string); ok && requestID != "" {
		return requestID
	}
	if requestID, ok := ctx.Value(config.HeaderKey).(string); ok && requestID != "" {
		return requestID
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(config.HeaderKey); len(values) > 0 && values[0] != "" {
			return values[0]
		}
		if values := md.Get(service.HeaderKey); len(values) > 0 && values[0] != "" {
			return values[0]
		}
	}

	return uuid.New().String()
}

func mapDeclensionResults(results []models.Result) []*v1.DeclensionResult {
	mapped := make([]*v1.DeclensionResult, 0, len(results))

	for _, result := range results {
		mapped = append(mapped, &v1.DeclensionResult{
			Word:        result.Word,
			Rule:        result.Rule,
			RootWord:    result.RootWord,
			Translation: result.Translation,
		})
	}

	return mapped
}

func mapGrammarAudit(audit *GrammarAuditLog) *v1.GrammarAudit {
	if audit == nil {
		return nil
	}

	events := make([]*v1.GrammarAuditEvent, 0, len(audit.Events))
	for _, event := range audit.Events {
		events = append(events, &v1.GrammarAuditEvent{
			Step:           event.Step,
			Status:         event.Status,
			Reason:         event.Reason,
			Source:         event.Source,
			Rule:           event.Rule,
			RootWord:       event.RootWord,
			SearchTerm:     event.SearchTerm,
			ResultCount:    uint32(event.ResultCount),
			CandidateCount: uint32(event.CandidateCount),
			Details:        event.Details,
		})
	}

	return &v1.GrammarAudit{
		RequestId: audit.RequestID,
		Word:      audit.Word,
		Outcome:   audit.Outcome,
		Source:    audit.DecisionSource,
		Reason:    audit.DecisionReason,
		Events:    events,
	}
}

func mapResearchConjugations(conjugations []*sv1.Conjugation) []*v1.Conjugation {
	mapped := make([]*v1.Conjugation, 0, len(conjugations))

	for _, conjugation := range conjugations {
		if conjugation == nil {
			continue
		}
		mapped = append(mapped, &v1.Conjugation{
			Word: conjugation.Word,
			Rule: conjugation.Rule,
		})
	}

	return mapped
}

func mapResearchResults(response *sv1.AnalyzeResponse) []*v1.AnalyzeResult {
	results := make([]*v1.AnalyzeResult, 0)

	if response == nil {
		return results
	}

	if response.DirectResult != nil {
		results = append(results, mapResearchAnalyzeResults(response.DirectResult.Texts)...)
	}

	results = append(results, mapResearchAnalyzeResults(response.Results)...)

	return results
}

func mapResearchAnalyzeResults(results []*sv1.AnalyzeResult) []*v1.AnalyzeResult {
	mapped := make([]*v1.AnalyzeResult, 0, len(results))

	for _, result := range results {
		if result == nil || result.Text == nil {
			continue
		}

		mapped = append(mapped, &v1.AnalyzeResult{
			ReferenceLink: result.ReferenceLink,
			Author:        result.Author,
			Book:          result.Book,
			Reference:     result.Reference,
			Text: &v1.Rhema{
				Greek:        result.Text.Greek,
				Translations: result.Text.Translations,
				Section:      result.Text.Section,
			},
		})
	}

	return mapped
}
