package main

import (
	"context"
	"time"

	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"

	"weave/core/mongodb"
	ratelimit "weave/packages/shared-ratelimit"
)

// runHealthLoop keeps healthServer's overall ("") status in sync with
// core's actual dependencies, rather than the static "always SERVING" a
// bare health.NewServer() defaults to — otherwise the k8s readiness/
// liveness probes wired to this service (infra/k8s/core.yaml) never catch
// a real Mongo/Redis outage, they just confirm the process is alive.
func runHealthLoop(healthServer *health.Server, limiter *ratelimit.Limiter) {
	const (
		interval = 10 * time.Second
		timeout  = 5 * time.Second
	)

	check := func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		status := healthv1.HealthCheckResponse_SERVING
		if !mongodb.Healthy {
			status = healthv1.HealthCheckResponse_NOT_SERVING
		}
		if err := limiter.Ping(ctx); err != nil {
			status = healthv1.HealthCheckResponse_NOT_SERVING
		}
		healthServer.SetServingStatus("", status)
	}

	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		check()
	}
}
