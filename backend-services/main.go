// backend-services is the ONE Go Connect/gRPC server that holds MongoDB
// credentials for the whole system (CLAUDE.md — the data trust boundary).
// It registers one RPC service per collection (rpc_services/<collection>) —
// see CLAUDE.md in this directory for the layout convention.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"servicesphere/backend-services/configs"
	"servicesphere/backend-services/gen/backend_services/data_access/v1/dataaccessv1connect"
	"servicesphere/backend-services/gen/database/v1/databasev1connect"
	"servicesphere/backend-services/health"
	"servicesphere/backend-services/mongodb"
	"servicesphere/backend-services/rpc_services/ticket"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	configs.InitializeConfig()
	mongodb.InitDatabase(ctx)
	go mongodb.StartHealthMonitor(ctx)

	mux := http.NewServeMux()
	mux.Handle(dataaccessv1connect.NewTicketServiceHandler(&ticket.Server{}, connect.WithInterceptors()))
	mux.Handle(databasev1connect.NewHealthServiceHandler(health.NewHandler()))

	srv := &http.Server{
		Addr: configs.Vars.ServerAddr,
		// h2c: plaintext HTTP/2 so gRPC works locally without TLS.
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("backend-services listening on %s (mongo db=%s)", configs.Vars.ServerAddr, configs.Vars.MongoDbName)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}
