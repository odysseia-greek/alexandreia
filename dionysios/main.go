package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/odysseia-greek/agora/plato/logging"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/odysseia-greek/alexandria/dionysios/grammar"
)

const standardPort = ":5000"

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

	srv := grammar.InitRoutes(dionysiosConfig)

	logging.Debug(fmt.Sprintf("%s : %s", "running on port", port))
	err = http.ListenAndServe(port, srv)
	if err != nil {
		panic(err)
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
