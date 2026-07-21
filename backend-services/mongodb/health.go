package mongodb

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// Healthy reflects the last Mongo ping result. Read by the Connect health
// handler (backend-services/health/handler.go); written only by StartHealthMonitor.
var Healthy atomic.Bool

// StartHealthMonitor pings MongoDB on an interval and updates Healthy. Runs
// until ctx is cancelled — call with `go mongodb.StartHealthMonitor(ctx)`.
func StartHealthMonitor(ctx context.Context) {
	db := Db.(*DbType)
	check := func() {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := db.MongoConn.Ping(pingCtx, nil); err != nil {
			slog.Warn("mongodb ping failed", "err", err)
			Healthy.Store(false)
			return
		}
		Healthy.Store(true)
	}

	check() // run immediately, then on interval
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
