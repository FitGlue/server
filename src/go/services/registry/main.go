package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/fitglue/server/src/go/internal/infra"
	"github.com/fitglue/server/src/go/internal/registry"
	pb "github.com/fitglue/server/src/go/pkg/types/pb/services/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083" // Default port for registry service
	}

	logger := infra.NewLogger()
	infra.InitSentry()

	// Initialize RegistryStore with static data
	store, err := registry.NewStaticStore()
	if err != nil {
		logger.Error(context.Background(), "Failed to initialize static registry store", "error", err)
		os.Exit(1)
	}

	svc := registry.NewService(store, logger)

	server := grpc.NewServer()
	pb.RegisterRegistryServiceServer(server, svc)

	healthcheck := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthcheck)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	logger.Info(context.Background(), "Starting service.registry", "port", port)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
