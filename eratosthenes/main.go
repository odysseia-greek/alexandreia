package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	v1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	"github.com/odysseia-greek/alexandreia/eratosthenes/scholar"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const standardPort = ":50060"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = standardPort
	}
	//https://patorjk.com/software/taag/#p=display&f=Crawford2&t=eratosthenes&x=none&v=4&h=4&w=80&we=false
	logging.System(`
   ___  ____    ____  ______   ___   _____ ______  __ __    ___  ____     ___  _____
  /  _]|    \  /    ||      | /   \ / ___/|      ||  |  |  /  _]|    \   /  _]/ ___/
 /  [_ |  D  )|  o  ||      ||     (   \_ |      ||  |  | /  [_ |  _  | /  [_(   \_ 
|    _]|    / |     ||_|  |_||  O  |\__  ||_|  |_||  _  ||    _]|  |  ||    _]\__  |
|   [_ |    \ |  _  |  |  |  |     |/  \ |  |  |  |  |  ||   [_ |  |  ||   [_ /  \ |
|     ||  .  \|  |  |  |  |  |     |\    |  |  |  |  |  ||     ||  |  ||     |\    |
|_____||__|\_||__|__|  |__|   \___/  \___|  |__|  |__|__||_____||__|__||_____| \___|
`)
	logging.System("\"ὑποθεμένοις, ὥσπερ ἐκεῖνος, εἶναι τὸ μέγεθος τῆς γῆς σταδίων εἴκοσι πέντε μυριάδων καὶ δισχιλίων, ὡς καὶ Ἐρατοσθένης ἀποδίδωσιν·\"")
	logging.System("Assuming, as he does himself after the assertion of Eratosthenes, that the circumference of the earth is 252,000 stadia")
	logging.System("Strabo; Geography; 2.5.34")

	logging.System("starting up.....")
	logging.System("starting up and getting env variables")

	ctx := context.Background()
	cfg, err := scholar.CreateNewConfig(ctx)
	if err != nil {
		logging.Error(err.Error())
		log.Fatal("death has found me")
	}

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var server *grpc.Server

	server = grpc.NewServer(
		grpc.UnaryInterceptor(
			comedy.UnaryServerInterceptor(
				cfg.Streamer,
				comedy.WithHeaderKey(config.HeaderKey),
				comedy.WithContextKeyName(config.DefaultTracingName),
				comedy.WithCloseHop(),
			),
		),
	)

	reflection.Register(server)

	v1.RegisterAspasiaServiceServer(server, cfg)

	logging.Info(fmt.Sprintf("Server listening on %s", port))
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
