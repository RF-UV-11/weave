FROM golang:1.26-alpine AS build
WORKDIR /repo
COPY packages/shared-auth ./packages/shared-auth
COPY core/go.mod core/go.sum ./core/
WORKDIR /repo/core
RUN go mod download
COPY core/ .
RUN CGO_ENABLED=0 go build -o /out/core .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/core /core
EXPOSE 9090
ENTRYPOINT ["/core"]
