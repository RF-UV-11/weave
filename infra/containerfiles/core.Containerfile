FROM golang:1.26-alpine AS build
WORKDIR /repo
COPY packages/ ./packages/
COPY core/go.mod core/go.sum ./core/
WORKDIR /repo/core
RUN go mod download
COPY core/ .
RUN CGO_ENABLED=0 go build -o /out/core .
# grpc_health_probe: distroless has no shell, so a container-level
# healthcheck (podman-compose, and any non-k8s deployment) needs a static
# binary that speaks the standard gRPC health protocol rather than a CMD
# shell script.
RUN CGO_ENABLED=0 go install github.com/grpc-ecosystem/grpc-health-probe@v0.4.28

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/core /core
COPY --from=build /go/bin/grpc-health-probe /grpc-health-probe
EXPOSE 9090
ENTRYPOINT ["/core"]
