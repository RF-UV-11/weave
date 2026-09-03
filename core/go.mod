module weave/core

go 1.26.5

require (
	github.com/oklog/ulid/v2 v2.1.2
	go.mongodb.org/mongo-driver/v2 v2.8.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/qdrant/go-client v1.19.0 // indirect
	github.com/redis/go-redis/v9 v9.22.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
)

require (
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.52.0
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	weave/packages/shared-auth v0.0.0
	weave/packages/shared-ratelimit v0.0.0
)

replace weave/packages/shared-auth => ../packages/shared-auth

replace weave/packages/shared-ratelimit => ../packages/shared-ratelimit
