package grammar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/aristoteles"
	elastic "github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/aristoteles/models"
	"github.com/odysseia-greek/agora/eupalinos/stomion"
	eupalinospb "github.com/odysseia-greek/agora/eupalinos/v1"
	"github.com/odysseia-greek/agora/hesiodos"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	plato "github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/agora/plato/service"
	aristarchos "github.com/odysseia-greek/alexandreia/aristarchos/scholar"
	"github.com/odysseia-greek/alexandreia/eratosthenes/library"
	"github.com/odysseia-greek/alexandreia/kallimachos/scholia"
	aristophanes "github.com/odysseia-greek/attike/aristophanes/comedy"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"github.com/odysseia-greek/delphi/aristides/diplomat"
	pb "github.com/odysseia-greek/delphi/aristides/proto"
	"google.golang.org/grpc/metadata"
)

const (
	defaultIndex string = "grammar"
)

func CreateNewConfig(ctx context.Context) (*DionysosHandler, error) {
	tls := config.BoolFromEnv(config.EnvTlSKey)

	tracer, err := aristophanes.NewClientTracer(aristophanes.DefaultAddress)
	if err != nil {
		logging.Error(err.Error())
	}

	healthy := tracer.WaitForHealthyState()
	if !healthy {
		logging.Debug("tracing service not ready - restarting seems the only option")
		os.Exit(1)
	}

	streamer, err := tracer.Chorus(ctx)
	if err != nil {
		logging.Error(err.Error())
	}

	ambassador, err := diplomat.NewClientAmbassador(diplomat.DEFAULTADDRESS)
	ambassadorHealthy := ambassador.WaitForHealthyState()
	if !ambassadorHealthy {
		logging.Info("ambassador service not ready - restarting seems the only option")
		os.Exit(1)
	}

	traceID := uuid.New().String()
	spanID := aristophanes.GenerateSpanID()
	combinedID := fmt.Sprintf("%s+%s+%d", traceID, spanID, 1)

	ambassadorCtx, ctxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer ctxCancel()

	payload := &arv1.ObserveTraceStart{
		Method:        "GetSecret",
		Url:           diplomat.DEFAULTADDRESS,
		Host:          "",
		RemoteAddress: "",
		Operation:     "/delphi_ptolemaios.Ptolemaios/GetSecret",
	}

	go func() {
		parabasis := &arv1.ObserveRequest{
			TraceId:      traceID,
			ParentSpanId: spanID,
			SpanId:       spanID,
			Kind: &arv1.ObserveRequest_TraceStart{
				TraceStart: payload,
			},
		}
		if err := streamer.Send(parabasis); err != nil {
			logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
		}

		logging.Trace(fmt.Sprintf("trace with requestID: %s and span: %s", traceID, spanID))
	}()

	md := metadata.New(map[string]string{service.HeaderKey: combinedID})
	ambassadorCtx = metadata.NewOutgoingContext(ambassadorCtx, md)
	vaultConfig, err := ambassador.GetSecret(ambassadorCtx, &pb.VaultRequest{})
	if err != nil {
		logging.Error(err.Error())
		return nil, err
	}

	go func() {
		parabasis := &arv1.ObserveRequest{
			TraceId:      traceID,
			ParentSpanId: spanID,
			SpanId:       spanID,
			Kind: &arv1.ObserveRequest_TraceStop{
				TraceStop: &arv1.ObserveTraceStop{
					ResponseBody: fmt.Sprintf("user retrieved from vault: %s", vaultConfig.ElasticUsername),
				},
			},
		}

		err := streamer.Send(parabasis)
		if err != nil {
			logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
		}

		logging.Trace(fmt.Sprintf("trace closed with id: %s", traceID))
	}()

	elasticService := aristoteles.ElasticService(tls)

	cfg := models.Config{
		Service:     elasticService,
		Username:    vaultConfig.ElasticUsername,
		Password:    vaultConfig.ElasticPassword,
		ElasticCERT: vaultConfig.ElasticCERT,
	}

	elastic, err := aristoteles.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	index := config.StringFromEnv(config.EnvIndex, defaultIndex)
	cache, err := archytas.CreateBadgerClientWithOptions(archytas.WithLogging(false))
	if err != nil {
		return nil, err
	}

	client, err := config.CreateOdysseiaClient()
	if err != nil {
		return nil, err
	}

	logging.Debug("creating new aggregator client")
	aggregatorAddress := config.StringFromEnv(config.EnvAggregatorAddress, config.DefaultAggregatorAddress)
	aggregator, err := aristarchos.NewClientAggregator(aggregatorAddress)
	if err != nil {
		logging.Error(err.Error())
		return nil, err
	}
	aggregatorHealthy := aggregator.WaitForHealthyState()
	if !aggregatorHealthy {
		logging.Debug("aggregator service not ready - restarting seems the only option")
		os.Exit(1)
	}

	logging.Debug("aggregator client created and healthy")

	eupalinosAddress := config.StringFromEnv(config.EnvEupalinosService, config.DefaultEupalinosService)
	queue, err := stomion.NewEupalinosClient(eupalinosAddress)
	if err != nil {
		logging.Error(err.Error())
		return nil, err
	}
	if err := waitForEupalinos(ctx, queue, eupalinosAddress); err != nil {
		return nil, err
	}
	aggregatorChannel := config.StringFromEnv(config.EnvChannel, defaultAggregatorChannel)

	libraryClientAddress := config.StringFromEnv("ERATOSTHENES_SERVICE", "eratosthenes:50060")
	libraryClient, err := hesiodos.NewGenericGrpcClient[*library.LibraryClient](
		libraryClientAddress,
		library.NewEratosthenesClient,
	)

	if err != nil {
		logging.Error(err.Error())
	}

	scholarClientAddress := config.StringFromEnv("KALLIMACHOS_SERVICE", "kallimachos:50060")
	scholarClient, err := hesiodos.NewGenericGrpcClient[*scholia.ScholarClient](
		scholarClientAddress,
		scholia.NewKallimachosClient,
	)

	ctx, cancel := context.WithCancel(ctx)

	version := os.Getenv(config.EnvVersion)

	return &DionysosHandler{
		Elastic:           elastic,
		Cache:             cache,
		Index:             index,
		Version:           version,
		Client:            client,
		DeclensionConfig:  plato.DeclensionConfig{},
		LibraryService:    libraryClient,
		ScholarService:    scholarClient,
		Streamer:          streamer,
		AggregatorQueue:   queue,
		AggregatorChannel: aggregatorChannel,
		AggregatorClient:  aggregator,
		StreamerCancel:    cancel,
	}, nil
}

type eupalinosHealthClient interface {
	Health(context.Context, *eupalinospb.HealthRequest) (*eupalinospb.HealthResponse, error)
}

func waitForEupalinos(parent context.Context, client eupalinosHealthClient, address string) error {
	const (
		startupTimeout = 30 * time.Second
		attemptTimeout = 2 * time.Second
		retryInterval  = time.Second
	)

	ctx, cancel := context.WithTimeout(parent, startupTimeout)
	defer cancel()
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		attemptCtx, attemptCancel := context.WithTimeout(ctx, attemptTimeout)
		response, err := client.Health(attemptCtx, &eupalinospb.HealthRequest{})
		attemptCancel()
		if err == nil && response.GetHealthy() {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("health response reported healthy=false")
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("eupalinos at %q was not healthy within %s: %w", address, startupTimeout, lastErr)
		case <-ticker.C:
		}
	}
}

func QueryRuleSet(ctx context.Context, es elastic.Client, index string) (*plato.DeclensionConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if es == nil {
		mockElastic, err := elastic.NewMockClient("declensionsDionysos", http.StatusOK)
		if err != nil {
			return nil, err
		}
		es = mockElastic
	}

	query := es.Builder().MatchAll()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	response, err := es.Query().MatchWithScrollWithContext(ctx, index, query)

	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	logging.Debug(fmt.Sprintf("query response yielded: %v hits", len(response.Hits.Hits)))

	var declensionConfig plato.DeclensionConfig
	for _, jsonHit := range response.Hits.Hits {
		byteJson, err := json.Marshal(jsonHit.Source)
		if err != nil {
			return nil, err
		}
		var declension plato.Declension
		err = json.Unmarshal(byteJson, &declension)
		if err != nil {
			return nil, err
		}

		declensionConfig.Declensions = append(declensionConfig.Declensions, declension)
	}

	return &declensionConfig, nil

}
