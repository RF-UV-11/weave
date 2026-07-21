// Package database is the ONLY place in the repo allowed to hold a MongoDB
// connection. Every other service group reaches data through a data-access
// Connect/gRPC RPC — never a direct driver import. See CLAUDE.md.
package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Connect dials MongoDB and returns a ready client. Callers own its lifecycle
// and must call client.Disconnect(ctx) on shutdown.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(connectCtx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	return client, nil
}
