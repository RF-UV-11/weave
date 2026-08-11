package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"weave/core/configs"
	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
	"weave/core/rpc_services/tenant"
)

func main() {
	cfg := configs.Load()

	if err := mongodb.InitDatabase(cfg.MongoURI, cfg.MongoDBName); err != nil {
		log.Fatalf("mongodb: %v", err)
	}

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	dataaccessv1.RegisterTenantServiceServer(grpcServer, tenant.NewServer())

	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	log.Printf("weave/core listening on %s", cfg.GRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
