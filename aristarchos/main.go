package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/odysseia-greek/agora/plato/logging"
	pb "github.com/odysseia-greek/olympia/aristarchos/proto"
	"github.com/odysseia-greek/olympia/aristarchos/scholar"
	"google.golang.org/grpc"
)

const standardPort = ":50060"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = standardPort
	}

	//https://patorjk.com/software/taag/#p=display&f=Crawford2&t=ARISTARCHOS
	logging.System(`
  ____  ____   ____ _____ ______   ____  ____      __  __ __   ___   _____
 /    ||    \ |    / ___/|      | /    ||    \    /  ]|  |  | /   \ / ___/
|  o  ||  D  ) |  (   \_ |      ||  o  ||  D  )  /  / |  |  ||     (   \_ 
|     ||    /  |  |\__  ||_|  |_||     ||    /  /  /  |  _  ||  O  |\__  |
|  _  ||    \  |  |/  \ |  |  |  |  _  ||    \ /   \_ |  |  ||     |/  \ |
|  |  ||  .  \ |  |\    |  |  |  |  |  ||  .  \\     ||  |  ||     |\    |
|__|__||__|\_||____|\___|  |__|  |__|__||__|\_| \____||__|__| \___/  \___|

`)
	logging.System("\"̓Αρίσταρχος δὲ ό Σάμιος ὑποθεσίων τινων ἐξέδωκεν γραφάς, ἐν αἷς ἐκ τῶν ὑποκειμένων συμβαίνει τὸν κόσμον πολλαπλάσιον εἶμεν τοῦ νῦν εἰρημένου’\"")
	logging.System("\"But Aristarchus has brought out a book consisting of certain hypotheses, wherein it appears, as a consequence of the assumptions made, that the universe is many times greater than the 'universe' just mentioned\"")
	logging.System("starting up.....")
	logging.System("starting up and getting env variables")

	ctx := context.Background()
	config, err := scholar.CreateNewConfig(ctx)
	if err != nil {
		logging.Error(err.Error())
		log.Fatal("death has found me")
	}

	err = scholar.CreateIndexAtStartup(config.PolicyName, config.Index, config.Elastic)
	if err != nil {
		logging.Error(err.Error())
	}
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var server *grpc.Server

	server = grpc.NewServer(grpc.UnaryInterceptor(scholar.AggregatorInterceptor))

	pb.RegisterAristarchosServer(server, config)

	logging.Info(fmt.Sprintf("Server listening on %s", port))
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
