FROM golang:1.26-alpine
WORKDIR /src
COPY core/go.mod core/go.sum ./
RUN go mod download
COPY core/ ./
ENTRYPOINT ["go", "test", "./...", "-v"]
