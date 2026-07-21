// Command server is the backend-services entrypoint: the ONE Go Connect/gRPC
// server that holds MongoDB credentials for the whole system (CLAUDE.md — the
// data trust boundary). It registers one handler per data-access domain.
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

	"servicesphere/backend-services/data-access/ticket"
	"servicesphere/backend-services/database"
	"servicesphere/backend-services/database/migrate"
	"servicesphere/backend-services/database/repositories"
	"servicesphere/backend-services/gen/backend_services/data_access/v1/dataaccessv1connect"
	"servicesphere/backend-services/gen/database/v1/databasev1connect"
	"servicesphere/backend-services/internal/health"
)

const shutdownTimeout = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mongoURI := getenv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getenv("MONGO_DB_NAME", "servicesphere")
	addr := getenv("BACKEND_SERVICES_ADDR", ":8081")

	client, err := database.Connect(ctx, mongoURI)
	if err != nil {
		log.Fatalf("connect to mongo: %v", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("mongo disconnect: %v", err)
		}
	}()

	db := client.Database(dbName)
	if err := migrate.EnsureIndexes(ctx, db); err != nil {
		log.Fatalf("ensure indexes: %v", err)
	}

	ticketRepo := repositories.NewTicketRepository(db)
	ticketHandler := ticket.NewHandler(ticketRepo)
	healthHandler := health.NewHandler()

	mux := http.NewServeMux()
	mux.Handle(dataaccessv1connect.NewTicketServiceHandler(ticketHandler, connect.WithInterceptors()))
	mux.Handle(databasev1connect.NewHealthServiceHandler(healthHandler))

	srv := &http.Server{
		Addr: addr,
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

	log.Printf("backend-services listening on %s (mongo db=%s)", addr, dbName)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
