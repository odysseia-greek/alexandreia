package grammar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/middleware"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/agora/plato/service"
	pba "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	aristarchos "github.com/odysseia-greek/alexandreia/aristarchos/scholar"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	pb "github.com/odysseia-greek/attike/aristophanes/proto"
	"google.golang.org/grpc/metadata"
)

type DionysosHandler struct {
	Elastic          aristoteles.Client
	Cache            archytas.Client
	Index            string
	Client           service.OdysseiaClient
	DeclensionConfig models.DeclensionConfig
	Streamer         pb.TraceService_ChorusClient
	Aggregator       pba.Aristarchos_CreateNewEntryClient
	StreamerCancel   context.CancelFunc
	AggregatorCancel context.CancelFunc
	AggregatorClient *aristarchos.ClientAggregator
}

// PingPong pongs the ping
func (d *DionysosHandler) pingPong(w http.ResponseWriter, req *http.Request) {
	pingPong := models.ResultModel{Result: "pong"}
	middleware.ResponseWithJson(w, pingPong)
}

// returns the health of the api
func (d *DionysosHandler) health(w http.ResponseWriter, req *http.Request) {
	elasticHealth := d.Elastic.Health().Info()
	dbHealth := models.DatabaseHealth{
		Healthy:       elasticHealth.Healthy,
		ClusterName:   elasticHealth.ClusterName,
		ServerName:    elasticHealth.ServerName,
		ServerVersion: elasticHealth.ServerVersion,
	}
	healthy := models.Health{
		Healthy:  dbHealth.Healthy,
		Time:     time.Now().String(),
		Database: dbHealth,
	}
	if !healthy.Healthy {
		middleware.ResponseWithCustomCode(w, http.StatusBadGateway, healthy)
		return
	}
	middleware.ResponseWithJson(w, healthy)
}

func (d *DionysosHandler) checkGrammar(w http.ResponseWriter, req *http.Request) {
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

	queryWord := req.URL.Query().Get("word")

	if queryWord == "" {
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

	cacheItem, _ := d.Cache.Read(queryWord)
	if cacheItem != nil {
		var cache models.DeclensionTranslationResults
		err := json.Unmarshal(cacheItem, &cache)
		if err != nil {
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
				parabasis := &pb.ParabasisRequest{
					TraceId:      traceID,
					ParentSpanId: spanID,
					SpanId:       comedy.GenerateSpanID(),
					RequestType: &pb.ParabasisRequest_Span{
						Span: &pb.SpanRequest{
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
		err = d.sendWordsToAggregator(&cache, requestId)
		if err != nil {
			logging.Error(err.Error())
		}
		middleware.ResponseWithJson(w, cache)
		return
	}

	//check first if the word is part of aristarchos and if it exists there return the result from it
	aggrCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	md := metadata.New(map[string]string{service.HeaderKey: requestId})
	aggrCtx = metadata.NewOutgoingContext(context.Background(), md)
	aggregatorRequest := pba.AggregatorRequest{RootWord: queryWord}
	entry, err := d.AggregatorClient.RetrieveRootFromGrammarForm(aggrCtx, &aggregatorRequest)
	if err != nil {
		logging.Error(err.Error())
	}

	if entry != nil {
		declensionFromAggregator := models.DeclensionTranslationResults{Results: []models.Result{
			{
				Word:        entry.Word,
				Rule:        entry.Rule,
				RootWord:    entry.RootWord,
				Translation: entry.Translation,
			},
		}}

		stringifiedDeclension, _ := json.Marshal(declensionFromAggregator)
		ttl := time.Hour
		err = d.Cache.SetWithTTL(queryWord, string(stringifiedDeclension), ttl)

		if err != nil {
			logging.Error(fmt.Sprintf("error setting cache: %s", err.Error()))
		}

		middleware.ResponseWithJson(w, declensionFromAggregator)

	}

	declensions, _ := d.StartFindingRules(queryWord, requestId)
	if len(declensions.Results) == 0 || declensions.Results == nil {
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

	err = d.sendWordsToAggregator(declensions, requestId)
	if err != nil {
		logging.Error(fmt.Sprintf("error in aggregator: %s", err.Error()))
	}

	stringifiedDeclension, _ := json.Marshal(declensions)
	ttl := time.Hour
	err = d.Cache.SetWithTTL(queryWord, string(stringifiedDeclension), ttl)

	if err != nil {
		logging.Error(fmt.Sprintf("error setting cache: %s", err.Error()))
	}

	middleware.ResponseWithJson(w, *declensions)
}

func (d *DionysosHandler) sendWordsToAggregator(declensions *models.DeclensionTranslationResults, requestID string) error {
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

		if err := d.Aggregator.Send(request); err != nil {
			d.reestablishStream()
			if err = d.Aggregator.Send(request); err != nil {
				return err
			}
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
