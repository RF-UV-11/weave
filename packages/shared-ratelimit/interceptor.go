package ratelimit

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// MethodLimits maps a full gRPC method name (info.FullMethod, e.g.
// "/core.data_access.v1.AuthService/Login") to a limit stricter or looser
// than the default — Login/Register need much tighter limits than a
// generic read RPC since they're the classic brute-force/spam target.
type MethodLimits map[string]Config

// UnaryServerInterceptor rate-limits every RPC except those in exempt
// (health checks, reflection — needed for grpcurl/infra healthchecks to
// keep working) by client IP, so it protects unauthenticated RPCs
// (Login, Register) exactly as much as authenticated ones: this runs
// ahead of any auth interceptor in the chain, deliberately, since an
// attacker flooding Login has no token yet. A method not present in
// limits falls back to defaultLimit.
func UnaryServerInterceptor(l *Limiter, limits MethodLimits, defaultLimit Config, exempt ...string) grpc.UnaryServerInterceptor {
	skip := make(map[string]bool, len(exempt))
	for _, m := range exempt {
		skip[m] = true
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skip[info.FullMethod] {
			return handler(ctx, req)
		}

		cfg, ok := limits[info.FullMethod]
		if !ok {
			cfg = defaultLimit
		}

		key := fmt.Sprintf("ratelimit:%s:%s", info.FullMethod, clientKey(ctx))
		allowed, err := l.Allow(ctx, key, cfg)
		if err != nil {
			// Fail open: a Redis outage should degrade to "unprotected,"
			// not "core is entirely down." Rate limiting is defense in
			// depth, not the only line of defense.
			return handler(ctx, req)
		}
		if !allowed {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded for %s, try again later", info.FullMethod)
		}

		return handler(ctx, req)
	}
}

// clientKey identifies the caller for rate-limiting purposes: the TCP
// peer address gRPC itself observed.
//
// Deliberately does NOT read "x-forwarded-for" or any other client-
// supplied metadata — that header is set by the caller, so trusting it
// would let an attacker rotate a fake value on every request and evade
// the limit entirely, defeating the point. Known gap: once core sits
// behind a real reverse proxy in production (docs/architecture/
// ARCHITECTURE.md's Envoy grpc-web proxy), every caller behind it will
// share the proxy's peer address unless the proxy is configured to
// forward a verified client IP over a trusted channel (e.g. PROXY
// protocol) — that wiring is deployment infra, not something core can
// assume here, so it's tracked as a follow-up rather than solved by
// trusting an unverifiable header today.
func clientKey(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return "unknown"
}
