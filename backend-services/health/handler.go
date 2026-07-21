// Package health implements the shared HealthService (protos/database/v1/health.proto)
// that every Connect server in every service group exposes. Status reflects
// mongodb.Healthy, updated by mongodb.StartHealthMonitor's ping loop.
package health

import (
	"context"

	"connectrpc.com/connect"

	databasev1 "servicesphere/backend-services/gen/database/v1"
	"servicesphere/backend-services/mongodb"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Check(ctx context.Context, req *connect.Request[databasev1.CheckRequest]) (*connect.Response[databasev1.CheckResponse], error) {
	status := databasev1.CheckResponse_STATUS_NOT_SERVING
	if mongodb.Healthy.Load() {
		status = databasev1.CheckResponse_STATUS_SERVING
	}
	return connect.NewResponse(&databasev1.CheckResponse{Status: status}), nil
}
