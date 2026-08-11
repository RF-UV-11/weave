FROM golang:1.26-alpine
WORKDIR /repo
COPY packages/shared-auth ./packages/shared-auth
COPY core/go.mod core/go.sum ./core/
WORKDIR /repo/core
RUN go mod download
COPY core/ .
ENTRYPOINT ["go", "test", "./...", "-v"]
