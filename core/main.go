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
	"weave/core/rpc_services/auth"
	"weave/core/rpc_services/bot_profile"
	"weave/core/rpc_services/connector"
	"weave/core/rpc_services/tenant"
	"weave/core/vault"
	sharedauth "weave/packages/shared-auth"
)

// unauthenticatedMethods are exempt from the JWT interceptor: the auth
// bootstrap RPCs themselves (can't require a token to log in), health/
// reflection (grpcurl and infra healthchecks need these to work at all),
// and every existing Tenant/Connector RPC, which predate Phase 2's auth
// wiring and aren't retrofitted with tenant/role checks yet — tracked as a
// follow-up in PLAN.md rather than silently left open.
var unauthenticatedMethods = []string{
	"/core.data_access.v1.AuthService/Register",
	"/core.data_access.v1.AuthService/Login",
	"/core.data_access.v1.AuthService/Refresh",
	"/core.data_access.v1.TenantService/CreateTenant",
	"/core.data_access.v1.TenantService/GetTenant",
	"/core.data_access.v1.TenantService/ListTenants",
	"/core.data_access.v1.ConnectorService/RegisterConnector",
	"/core.data_access.v1.ConnectorService/ListConnectors",
	"/core.data_access.v1.ConnectorService/RefreshManifest",
	"/core.data_access.v1.ConnectorService/DeregisterConnector",
	"/grpc.health.v1.Health/Check",
	"/grpc.health.v1.Health/Watch",
	"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
	"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
}

func main() {
	cfg := configs.Load()

	if err := mongodb.InitDatabase(cfg.MongoURI, cfg.MongoDBName); err != nil {
		log.Fatalf("mongodb: %v", err)
	}

	v, err := vault.New(cfg.VaultRootKey)
	if err != nil {
		log.Fatalf("vault: %v", err)
	}

	if cfg.JWTSecret == "" {
		log.Fatalf("auth: JWT_SECRET is required")
	}
	jwtSecret := []byte(cfg.JWTSecret)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(sharedauth.UnaryServerInterceptor(jwtSecret, unauthenticatedMethods...)),
	)
	dataaccessv1.RegisterTenantServiceServer(grpcServer, tenant.NewServer())
	dataaccessv1.RegisterConnectorServiceServer(grpcServer, connector.NewServer(v))
	dataaccessv1.RegisterAuthServiceServer(grpcServer, auth.NewServer(jwtSecret))
	dataaccessv1.RegisterBotProfileServiceServer(grpcServer, botprofile.NewServer())

	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	log.Printf("weave/core listening on %s", cfg.GRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
