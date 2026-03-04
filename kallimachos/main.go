package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	v1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	"github.com/odysseia-greek/alexandreia/kallimachos/library"
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

	// https://patorjk.com/software/taag/#p=display&f=Crawford2&t=Kallimachos&x=none&v=4&h=4&w=80&we=false
	logging.System(`
 __  _   ____  _      _      ____  ___ ___   ____    __  __ __   ___   _____
|  |/ ] /    || |    | |    |    ||   |   | /    |  /  ]|  |  | /   \ / ___/
|  ' / |  o  || |    | |     |  | | _   _ ||  o  | /  / |  |  ||     (   \_ 
|    \ |     || |___ | |___  |  | |  \_/  ||     |/  /  |  _  ||  O  |\__  |
|     ||  _  ||     ||     | |  | |   |   ||  _  /   \_ |  |  ||     |/  \ |
|  .  ||  |  ||     ||     | |  | |   |   ||  |  \     ||  |  ||     |\    |
|__|\_||__|__||_____||_____||____||___|___||__|__|\____||__|__| \___/  \___|
`)
	logging.System("\"μέγα βιβλίον, μέγα κακόν\"")
	logging.System("Big book, big evil")
	logging.System("Kallimachos, fragment 465")

	ctx := context.Background()
	cfg, err := library.CreateNewConfig(ctx)
	if err != nil {
		logging.Error(err.Error())
		log.Fatal("death has found me")
	}

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer(
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
	v1.RegisterKallimachosServiceServer(server, cfg)

	logging.Info(fmt.Sprintf("Server listening on %s", port))
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
