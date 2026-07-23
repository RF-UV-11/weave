# backend-services

Central Connect/gRPC server over MongoDB. Every other service group (`ai-services`,
`analysis-services`, `channels`, `mcp-servers`) reaches the database through this
service's RPCs — they never open a Mongo connection directly. See root `CLAUDE.md`
("data trust boundary") and `docs/architecture/OVERVIEW.md` §5.1.

## Run / test
- Run: `go run .` from this dir (needs MongoDB reachable at `MONGO_URI`).
- Test: `go test ./...` from this dir, or `make test-backend` from the repo root.
- Config: `configs/configs.go`, env vars documented in `.env.example` at the repo root.

## Layout (RPC-per-collection)
Each MongoDB collection is exposed as exactly one Connect/gRPC service:
- `mongodb/<collection>.go` — interface + `DbType` methods (the only place that touches Mongo). Stores the `database/v1` proto message directly as the document (BSON via its `json` struct tags, `UseJSONStructTags` in `initialize.go`) — no separate DTO type. The entity's `_id` field means Mongo's real primary key IS the ID; in Go that's `t.XId`/`t.GetXId()` (protoc-gen-go's mangled name for a leading-underscore field) — always use that, never add a second app-level ID field.
- `rpc_services/<collection>/server.go` — struct embedding the Connect `Unimplemented*Handler` stub.
- `rpc_services/<collection>/routehandler.go` — RPC methods that **delegate to the mongodb layer; no business logic here.**
- `mongodb/initialize.go` — wires `Queries`, the `DbType` collection fields, and `InitDatabase()`.
- `mongodb/collections.go` — `ColNames`, the single registry of collection name strings.
- `mongodb/indexes.go` — index bootstrap (Mongo is schemaless, so this is the migration story — idempotent, runs on every startup).
- `mongodb/health.go` — periodic Mongo ping loop; `health/handler.go` is the thin Connect adapter that reads it.

## Adding a new collection RPC
1. `protos/database/v1/<collection>.proto` — the entity, marked `is_collection: true` (see root `CLAUDE.md`).
2. `protos/backend_services/data_access/v1/<collection>.proto` — RPC-only Request/Response + service, importing the entity.
3. `buf generate`.
4. `mongodb/<collection>.go` — interface + `DbType` methods; embed the interface into `Queries` in `initialize.go`; add the collection name to `ColNames`; add its collection field to `DbType` and wire it in `InitDatabase()`.
5. `mongodb/indexes.go` — add its index block.
6. `rpc_services/<collection>/server.go` + `routehandler.go`.
7. Register the handler in `main.go`.
8. Add/extend a client in `packages/shared-clients` for the services that call it.

## Conventions
- Never run delete/drop/truncate against any database from code or a one-off script without an explicit, reviewed migration — see root `CLAUDE.md`.
- Through Phase 0, this server has no auth interceptor. Phase 1 builds `packages/shared-auth`'s JWT-verify Connect interceptor (used by the new `auth` domain itself); Phase 10 wires it into every other RPC on this server too. Until then, this server trusts callers within the compose/cluster network, same as `ai-services`' tool layer trusts its own RBAC check — don't treat the interim no-auth state as a permanent design choice.
