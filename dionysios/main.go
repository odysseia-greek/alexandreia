package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/models"
	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	"github.com/odysseia-greek/alexandreia/dionysios/grammar"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const standardPort = ":5000"
const standardGrpcPort = ":50060"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = standardPort
	}

	//https://patorjk.com/software/taag/#p=display&f=Crawford2&t=DIONYSIOS
	logging.System(`
 ___    ____  ___   ____   __ __  _____ ____  ___   _____
|   \  |    |/   \ |    \ |  |  |/ ___/|    |/   \ / ___/
|    \  |  ||     ||  _  ||  |  (   \_  |  ||     (   \_ 
|  D  | |  ||  O  ||  |  ||  ~  |\__  | |  ||  O  |\__  |
|     | |  ||     ||  |  ||___, |/  \ | |  ||     |/  \ |
|     | |  ||     ||  |  ||     |\    | |  ||     |\    |
|_____||____|\___/ |__|__||____/  \___||____|\___/  \___|
                                                         
`)
	logging.System("\"Γραμματική ἐστιν ἐμπειρία τῶν παρὰ ποιηταῖς τε καὶ συγγραφεῦσιν ὡς ἐπὶ τὸ πολὺ λεγομένων.’\"")
	logging.System("\"Grammar is an experimental knowledge of the usages of language as generally current among poets and prose writers\"")
	logging.System("starting up.....")
	logging.System("starting up and getting env variables")

	ctx := context.Background()
	dionysiosConfig, err := grammar.CreateNewConfig(ctx)
	if err != nil {
		logging.Error(err.Error())
		log.Fatal("death has found me")
	}

	declensionConfig, err := grammar.QueryRuleSet(dionysiosConfig.Elastic, dionysiosConfig.Index)
	if err != nil {
		logging.Error(err.Error())
		log.Fatal("death has found me")
	}
	dionysiosConfig.DeclensionConfig = *declensionConfig

	// Start a goroutine to periodically update the grammar config
	logging.Debug("starting goroutine to periodically update the grammar config")
	go updateGrammarConfig(dionysiosConfig)
	go startGrpcServer(dionysiosConfig)

	srv := grammar.InitRoutes(dionysiosConfig)

	logging.Debug(fmt.Sprintf("%s : %s", "running on port", port))
	err = http.ListenAndServe(port, srv)
	if err != nil {
		panic(err)
	}
}

func startGrpcServer(dionysiosConfig *grammar.DionysosHandler) {
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = standardGrpcPort
	}

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen for grpc: %v", err)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(
			comedy.UnaryServerInterceptor(
				dionysiosConfig.Streamer,
				comedy.WithHeaderKey(config.HeaderKey),
				comedy.WithContextKeyName(config.DefaultTracingName),
				comedy.WithCloseHop(),
			),
		),
	)

	reflection.Register(server)
	v1.RegisterDionysiosServiceServer(server, dionysiosConfig)

	logging.Info(fmt.Sprintf("gRPC server listening on %s", port))
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve grpc: %v", err)
	}
}

// updateGrammarConfig periodically fetches the grammar config from Elasticsearch
// and updates the provided dionysiosConfig if there is any difference.
func updateGrammarConfig(dionysiosConfig *grammar.DionysosHandler) {
	ticker := time.NewTicker(2 * time.Minute)
	for {
		select {
		case <-ticker.C:
			declensionConfig, err := grammar.QueryRuleSet(dionysiosConfig.Elastic, dionysiosConfig.Index)
			if err != nil {
				logging.Debug(fmt.Sprintf("failed to fetch updated declension config: %s", err.Error()))
				continue // Retry on the next tick
			}

			if !isSameDeclensionConfig(*declensionConfig, dionysiosConfig.DeclensionConfig) {
				logging.Debug("Detected a difference in the grammar config. Updating...")
				dionysiosConfig.DeclensionConfig = *declensionConfig
			}
		}
	}
}

// isSameDeclensionConfig checks if two DeclensionConfig structs are the same.
func isSameDeclensionConfig(config1, config2 models.DeclensionConfig) bool {
	config1JSON, _ := json.Marshal(config1)
	config2JSON, _ := json.Marshal(config2)
	return string(config1JSON) == string(config2JSON)
}
