package scholar

import (
	"context"
	"fmt"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	aristophanes "github.com/odysseia-greek/attike/aristophanes/comedy"
	pbar "github.com/odysseia-greek/attike/aristophanes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"strings"
)

func AggregatorInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	var requestId string

	headerValue := md.Get(config.HeaderKey)
	if len(headerValue) > 0 {
		requestId = headerValue[0]
	}

	if requestId == "" {
		return handler(ctx, req)
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

	host := ""
	if p, ok := peer.FromContext(ctx); ok {
		host = p.Addr.String()
	}

	if traceCall {
		newSpan := aristophanes.GenerateSpanID()
		combinedId := fmt.Sprintf("%s+%s+%d", traceID, spanID, 1)
		newCtx := context.WithValue(ctx, config.DefaultTracingName, combinedId)

		go func() {
			parabasis := &pbar.ParabasisRequest{
				TraceId:      traceID,
				ParentSpanId: spanID,
				SpanId:       newSpan,
				RequestType: &pbar.ParabasisRequest_Trace{
					Trace: &pbar.TraceRequest{
						Method: info.FullMethod,
						Url:    info.FullMethod,
						Host:   host,
					},
				},
			}
			if err := streamer.Send(parabasis); err != nil {
				logging.Error(fmt.Sprintf("failed to send trace data: %v", err))
			}

			logging.Trace(fmt.Sprintf("trace with requestID: %s and span: %s", requestId, newSpan))
		}()
		responseMd := metadata.New(map[string]string{config.HeaderKey: traceID})
		grpc.SendHeader(newCtx, responseMd)
		return handler(newCtx, req)
	}

	return handler(ctx, req)
}
