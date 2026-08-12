package grammar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/aristoteles"
	queuepb "github.com/odysseia-greek/agora/eupalinos/v1"
	"github.com/odysseia-greek/agora/hesiodos"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/middleware"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/agora/plato/service"
	pba "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	aristarchos "github.com/odysseia-greek/alexandreia/aristarchos/scholar"
	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	"github.com/odysseia-greek/alexandreia/eratosthenes/library"
	sv1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"github.com/odysseia-greek/alexandreia/kallimachos/scholia"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type DionysosHandler struct {
	Elastic           aristoteles.Client
	Cache             archytas.Client
	Index             string
	Version           string
	Client            service.OdysseiaClient
	LibraryService    *hesiodos.GenericGrpcClient[*library.LibraryClient]
	ScholarService    *hesiodos.GenericGrpcClient[*scholia.ScholarClient]
	DeclensionConfig  models.DeclensionConfig
	DeclensionMu      sync.RWMutex
	TextModeMu        sync.Mutex
	TextModeSessions  map[string]time.Time
	TextModeIPs       map[string]time.Time
	Streamer          arv1.TraceService_ChorusClient
	Aggregator        pba.Aristarchos_CreateNewEntryClient
	AggregatorQueue   AggregatorQueue
	AggregatorChannel string
	StreamerCancel    context.CancelFunc
	AggregatorCancel  context.CancelFunc
	AggregatorClient  *aristarchos.ClientAggregator
	v1.UnimplementedDionysiosServiceServer
}

type AggregatorQueue interface {
	WaitForHealthyState() bool
	EnqueueMessageBytes(ctx context.Context, in *queuepb.EpistelloBytes) (*queuepb.EnqueueResponse, error)
	DequeueMessageBytes(ctx context.Context, in *queuepb.ChannelInfo) (*queuepb.EpistelloBytes, error)
}

const defaultAggregatorChannel = "aristarchos"

func (d *DionysosHandler) outgoingCtx(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)

	reqID, _ := parent.Value(config.HeaderKey).(string)

	kvs := make([]string, 0, 4)

	if reqID != "" {
		kvs = append(kvs, config.HeaderKey, reqID)
	}

	if len(kvs) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, kvs...)
	}

	return ctx, cancel
}

func (d *DionysosHandler) checkGrammar(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	ctx := req.Context()
	var requestId string
	fromContext := req.Context().Value(config.DefaultTracingName)
	if fromContext == nil {
		requestId = req.Header.Get(config.HeaderKey)
	} else {
		requestId = fromContext.(string)
	}
	splitID := strings.Split(requestId, "+")

	traceCall := false
	var traceID, spanID string

	if len(splitID) >= 3 {
		traceCall = splitID[2] == "1"
	}

	if len(splitID) >= 1 {
		traceID = splitID[0]
	}
	if len(splitID) >= 2 {
		spanID = splitID[1]
	}

	ctx = context.WithValue(ctx, config.HeaderKey, requestId)

	queryWord := req.URL.Query().Get("word")
	includeAudit := auditRequested(req.URL.Query().Get("audit"))
	auditLog := newGrammarAuditLog(requestId, queryWord)
	auditLog.Add(GrammarAuditEvent{
		Step:   "request.received",
		Status: "ok",
		Reason: "grammar check request accepted",
	})

	writeResults := func(results *models.DeclensionTranslationResults) {
		auditLog.Emit()
		if includeAudit {
			middleware.ResponseWithCustomCode(w, http.StatusOK, GrammarAuditResponse{
				Results: results.Results,
				Audit:   *auditLog,
			})
			return
		}
		middleware.ResponseWithCustomCode(w, http.StatusOK, *results)
		return
	}

	if queryWord == "" {
		auditLog.Add(GrammarAuditEvent{
			Step:   "request.validation",
			Status: "failed",
			Reason: "word query parameter is empty",
		})
		auditLog.Complete("validation_error", "request", "missing word query parameter")
		auditLog.Emit()
		e := models.ValidationError{
			ErrorModel: models.ErrorModel{UniqueCode: traceID},
			Messages: []models.ValidationMessages{
				{
					Field:   "word",
					Message: "cannot be empty",
				},
			},
		}
		middleware.ResponseWithJson(w, e)
		return
	}

	var cacheItem []byte
	if d.Cache == nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.lookup",
			Status: "skipped",
			Reason: "cache client is not configured",
			Source: "cache",
		})
	} else {
		cacheItem, _ = d.Cache.Read(grammarCacheKey(queryWord))
		auditLog.Add(GrammarAuditEvent{
			Step:   "cache.lookup",
			Status: "ok",
			Reason: "checked grammar cache",
			Source: "cache",
			Details: []string{
				fmt.Sprintf("hit=%t", cacheItem != nil),
			},
		})
	}
	if cacheItem != nil {
		cache, cachedAudit, err := decodeGrammarCacheEntry(cacheItem)
		if err != nil {
			auditLog.Add(GrammarAuditEvent{
				Step:   "cache.unmarshal",
				Status: "failed",
				Reason: err.Error(),
				Source: "cache",
			})
			auditLog.Complete("error", "cache", "cached payload could not be parsed")
			auditLog.Emit()
			e := models.ValidationError{
				ErrorModel: models.ErrorModel{UniqueCode: traceID},
				Messages: []models.ValidationMessages{
					{
						Field:   "cache",
						Message: err.Error(),
					},
				},
			}

			if traceCall {
				parabasis := &arv1.ObserveRequest{
					TraceId:      traceID,
					ParentSpanId: spanID,
					SpanId:       comedy.GenerateSpanID(),
					Kind: &arv1.ObserveRequest_Action{
						Action: &arv1.ObserveAction{
							Action: "TakenFromCache",
							Status: fmt.Sprintf("status code: %d", http.StatusOK),
						},
					},
				}
				if err := d.Streamer.Send(parabasis); err != nil {
					logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
				}
			}
			middleware.ResponseWithJson(w, e)
			return
		}
		auditLog.AddCachedHistory(cachedAudit)
		auditLog.Add(GrammarAuditEvent{
			Step:        "cache.return",
			Status:      "ok",
			Reason:      "returning cached grammar result",
			Source:      "cache",
			ResultCount: len(cache.Results),
		})
		err = d.sendWordsToAggregator(ctx, cache, requestId)
		if err != nil {
			auditLog.Add(GrammarAuditEvent{
				Step:   "aggregator.send",
				Status: "failed",
				Reason: err.Error(),
				Source: "aggregator",
			})
			logging.Error(err.Error())
		} else {
			auditLog.Add(GrammarAuditEvent{
				Step:        "aggregator.send",
				Status:      "ok",
				Reason:      "cached result forwarded to aggregator",
				Source:      "aggregator",
				ResultCount: len(cache.Results),
			})
		}
		auditLog.Complete("success", "cache", "cache hit satisfied request")
		writeResults(cache)
		return
	}

	declensions, err := d.StartFindingRules(ctx, queryWord, auditLog)
	if err != nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "rule_engine.execute",
			Status: "failed",
			Reason: err.Error(),
			Source: "rule-engine",
		})
		auditLog.Complete("error", "rule-engine", "rule engine execution failed")
		auditLog.Emit()
		middleware.ResponseWithCustomCode(w, http.StatusInternalServerError, "something went wrong")
		return
	}
	if len(declensions.Results) == 0 || declensions.Results == nil {
		auditLog.Add(GrammarAuditEvent{
			Step:   "rule_engine.result",
			Status: "empty",
			Reason: "no declension or dictionary matches were found",
			Source: "rule-engine",
		})
		auditLog.Complete("not_found", "rule-engine", "no options found")
		auditLog.Emit()
		e := models.NotFoundError{
			ErrorModel: models.ErrorModel{UniqueCode: traceID},
			Message: models.NotFoundMessage{
				Type:   queryWord,
				Reason: "no options found",
			},
		}
		middleware.ResponseWithJson(w, e)
		return
	}

	err = d.sendWordsToAggregator(ctx, declensions, requestId)
	if err != nil {
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

	auditLog.Complete("success", "rule-engine", "rule engine generated the final result set")
	d.cacheGrammarResults(queryWord, declensions, auditLog)

	if traceCall {
		// this span is meant to give insight into the working of StartFindingRules and should be expanded
		duration := time.Since(startTime)
		status, err := json.Marshal(declensions)
		if err != nil {
			logging.Error(fmt.Sprintf("failed to marshal body: %v", err))
		}
		parabasis := &arv1.ObserveRequest{
			TraceId:      traceID,
			ParentSpanId: spanID,
			SpanId:       comedy.GenerateSpanID(),
			Kind: &arv1.ObserveRequest_Action{
				Action: &arv1.ObserveAction{
					Action: "StartFindingRules",
					TookMs: duration.Milliseconds(),
					Status: fmt.Sprintf("%s", string(status)),
				},
			},
		}
		if err := d.Streamer.Send(parabasis); err != nil {
			logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
		}
	}

	writeResults(declensions)
}

// analyseText fetches words and queries them in all the texts
func (d *DionysosHandler) researchWord(w http.ResponseWriter, req *http.Request) {
	var requestId string
	fromContext := req.Context().Value(config.DefaultTracingName)
	if fromContext == nil {
		requestId = req.Header.Get(config.HeaderKey)
	} else {
		requestId = fromContext.(string)
	}
	traceID := strings.Split(requestId, "+")[0]

	var analyzeTextRequest models.AnalyzeTextRequest
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&analyzeTextRequest)
	if err != nil {
		e := models.ValidationError{
			ErrorModel: models.ErrorModel{UniqueCode: traceID},
			Messages: []models.ValidationMessages{
				{
					Field:   "decoding",
					Message: err.Error(),
				},
			},
		}
		middleware.ResponseWithJson(w, e)
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 60*time.Second)
	defer cancel()
	md := metadata.New(map[string]string{service.HeaderKey: requestId})
	ctx = metadata.NewOutgoingContext(ctx, md)

	request := d.scholarAnalyzeRequest(ctx, analyzeTextRequest.Rootword, 5)
	results, err := d.ScholarService.Client.Analyze(ctx, request)
	if err != nil {
		middleware.ResponseWithCustomCode(w, http.StatusBadRequest, "something went wrong")
		return
	}

	result := models.AnalyzeTextResponse{
		Rootword:     results.Rootword,
		PartOfSpeech: results.PartOfSpeech,
		Conjugations: mapScholarConjugations(results.Conjugations),
		Results:      flattenScholarResults(results),
	}

	middleware.ResponseWithCustomCode(w, http.StatusOK, result)
}

func mapScholarConjugations(conjugations []*sv1.Conjugation) []models.Conjugations {
	mapped := make([]models.Conjugations, 0, len(conjugations))

	for _, conjugation := range conjugations {
		if conjugation == nil {
			continue
		}
		mapped = append(mapped, models.Conjugations{
			Word: conjugation.Word,
			Rule: conjugation.Rule,
		})
	}

	return mapped
}

func flattenScholarResults(response *sv1.AnalyzeResponse) []models.AnalyzeResult {
	results := make([]models.AnalyzeResult, 0)

	if response == nil {
		return results
	}

	if response.DirectResult != nil {
		results = append(results, mapScholarResults(response.DirectResult.Texts)...)
	}

	results = append(results, mapScholarResults(response.Results)...)

	return results
}

func mapScholarResults(results []*sv1.AnalyzeResult) []models.AnalyzeResult {
	mapped := make([]models.AnalyzeResult, 0, len(results))

	for _, result := range results {
		if result == nil || result.Text == nil {
			continue
		}

		mapped = append(mapped, models.AnalyzeResult{
			ReferenceLink: result.ReferenceLink,
			Author:        result.Author,
			Book:          result.Book,
			Reference:     result.Reference,
			Text: models.Rhema{
				Greek:        result.Text.Greek,
				Translations: result.Text.Translations,
				Section:      result.Text.Section,
			},
		})
	}

	return mapped
}

func (d *DionysosHandler) sendWordsToAggregator(ctx context.Context, declensions *models.DeclensionTranslationResults, requestID string) error {
	if d.AggregatorQueue == nil {
		return nil
	}

	aggregatorChannel := d.AggregatorChannel
	if aggregatorChannel == "" {
		aggregatorChannel = defaultAggregatorChannel
	}

	for _, declension := range declensions.Results {
		if len(declension.Translation) == 0 {
			continue
		}

		speech := pba.PartOfSpeech_VERB

		if strings.Contains(declension.Rule, "noun") {
			speech = pba.PartOfSpeech_NOUN
		}

		if declension.Rule == "participle" {
			speech = pba.PartOfSpeech_PARTICIPLE
		}

		if strings.Contains(declension.Rule, "adverb") {
			speech = pba.PartOfSpeech_ADVERB
		}

		if strings.Contains(declension.Rule, "conjunction") {
			speech = pba.PartOfSpeech_CONJUNCTION
		}

		if strings.Contains(declension.Rule, "preposition") {
			speech = pba.PartOfSpeech_PREPOSITION
		}

		if declension.Rule == "particle" {
			speech = pba.PartOfSpeech_PARTICLE
		}

		if strings.Contains(declension.Rule, "pronoun") {
			speech = pba.PartOfSpeech_PRONOUN
		}

		if strings.Contains(declension.Rule, "article") {
			speech = pba.PartOfSpeech_ARTICLE
			if strings.Contains(declension.RootWord, " ") {
				continue
			}
		}

		processedRootWord := d.processRootWord(declension.RootWord)

		request := &pba.AggregatorCreationRequest{
			Word:         declension.Word,
			Rule:         declension.Rule,
			RootWord:     processedRootWord,
			Translation:  declension.Translation[0],
			PartOfSpeech: speech,
			TraceId:      requestID,
		}

		data, err := proto.Marshal(request)
		if err != nil {
			return fmt.Errorf("marshal aggregator request: %w", err)
		}

		_, err = d.AggregatorQueue.EnqueueMessageBytes(ctx, &queuepb.EpistelloBytes{
			Channel: aggregatorChannel,
			Data:    data,
		})
		if err != nil {
			return fmt.Errorf("enqueue aggregator request: %w", err)
		}
	}
	return nil
}

func (d *DionysosHandler) processRootWord(rootWord string) string {
	if strings.Contains(rootWord, "–") {
		parts := strings.Split(rootWord, "–")
		if strings.Contains(parts[0], ",") {
			innerParts := strings.Split(parts[0], ",")
			return strings.TrimSpace(innerParts[0])
		}
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(rootWord)
}

func (d *DionysosHandler) reestablishStream() {
	logging.Debug("stream is invalid so resetting stream")
	if d.AggregatorCancel != nil {
		d.AggregatorCancel()
	}

	aggregatorAddress := config.StringFromEnv(config.EnvAggregatorAddress, config.DefaultAggregatorAddress)
	aggregator, err := aristarchos.NewClientAggregator(aggregatorAddress)
	if err != nil {
		logging.Error(err.Error())
		return
	}
	aggregatorHealthy := aggregator.WaitForHealthyState()
	if !aggregatorHealthy {
		logging.Error("aggregator service not ready")
		return
	}

	aggrContext, aggregatorCancel := context.WithCancel(context.Background())
	aristarchosStreamer, err := d.AggregatorClient.CreateNewEntry(aggrContext)
	if err != nil {
		logging.Error(err.Error())
		aggregatorCancel()
		return
	}

	d.Aggregator = aristarchosStreamer
	d.AggregatorCancel = aggregatorCancel
}
