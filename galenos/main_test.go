package main

import (
	"context"
	"os"
	"testing"
	"time"

	dionysiosv1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const defaultDionysiosAddress = "localhost:50060"

var (
	dionysiosClient dionysiosv1.DionysiosServiceClient
	connection      *grpc.ClientConn
)

func TestDionysios(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dionysios gRPC Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	address := os.Getenv("DIONYSIOS_GRPC_ADDRESS")
	if address == "" {
		address = defaultDionysiosAddress
	}

	var err error
	connection, err = grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(err).NotTo(HaveOccurred())
	dionysiosClient = dionysiosv1.NewDionysiosServiceClient(connection)

	Eventually(func(g Gomega) {
		callCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		response, callErr := dionysiosClient.Health(callCtx, &dionysiosv1.HealthRequest{})
		g.Expect(callErr).NotTo(HaveOccurred())
		g.Expect(response.GetHealthy()).To(BeTrue())
	}).WithTimeout(30 * time.Second).WithPolling(time.Second).Should(Succeed())
}, NodeTimeout(35*time.Second))

var _ = AfterSuite(func() {
	if connection != nil {
		Expect(connection.Close()).To(Succeed())
	}
})

func callContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
