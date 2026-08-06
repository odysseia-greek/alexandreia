package scholia

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/aristoteles/models"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/service"
	aristophanes "github.com/odysseia-greek/attike/aristophanes/comedy"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"github.com/odysseia-greek/delphi/aristides/diplomat"
	pb "github.com/odysseia-greek/delphi/aristides/proto"
	"google.golang.org/grpc/metadata"
)

const defaultIndex = "text"

func CreateNewConfig(ctx context.Context) (*ScholarServiceImpl, error) {
	tls := config.BoolFromEnv(config.EnvTlSKey)

	tracer, err := aristophanes.NewClientTracer(aristophanes.DefaultAddress)
	if err != nil {
		logging.Error(err.Error())
	}

	healthy := tracer.WaitForHealthyState()
	if !healthy {
		logging.Error("tracing service not ready - setting tracer to nil and starting backup process")
		tracer = nil
	}

	var streamer arv1.TraceService_ChorusClient
	if tracer != nil {
		streamer, err = tracer.Chorus(ctx)
		if err != nil {
			logging.Error(err.Error())
		}
	}

	ambassador, err := diplomat.NewClientAmbassador(diplomat.DEFAULTADDRESS)
	if err != nil {
		return nil, err
	}

	healthy = ambassador.WaitForHealthyState()
	if !healthy {
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

	if streamer != nil {
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
		}()
	}

	md := metadata.New(map[string]string{service.HeaderKey: combinedID})
	ambassadorCtx = metadata.NewOutgoingContext(ambassadorCtx, md)
	vaultConfig, err := ambassador.GetSecret(ambassadorCtx, &pb.VaultRequest{})
	if err != nil {
		logging.Error(err.Error())
		return nil, err
	}

	if streamer != nil {
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

			if err := streamer.Send(parabasis); err != nil {
				logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
			}
		}()
	}

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

	if err := aristoteles.HealthCheck(elastic); err != nil {
		return nil, err
	}

	index := config.StringFromEnv(config.EnvIndex, defaultIndex)

	version := os.Getenv(config.EnvVersion)

	return &ScholarServiceImpl{
		Index:    index,
		Elastic:  elastic,
		Streamer: streamer,
		Version:  version,
	}, nil
}
