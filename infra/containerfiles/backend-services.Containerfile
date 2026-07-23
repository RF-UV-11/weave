# syntax=docker/dockerfile:1
FROM docker.io/library/golang:1.26-alpine AS build
WORKDIR /src
COPY backend-services/go.mod backend-services/go.sum ./backend-services/
WORKDIR /src/backend-services
RUN go mod download
WORKDIR /src
COPY backend-services ./backend-services
WORKDIR /src/backend-services
RUN CGO_ENABLED=0 go build -o /out/backend-services .

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/backend-services /usr/local/bin/backend-services
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/backend-services"]
