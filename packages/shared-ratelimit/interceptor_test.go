package ratelimit

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func withPeer(ctx context.Context, addr string) context.Context {
	return peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP(addr), Port: 12345}})
}

func okHandler(ctx context.Context, req any) (any, error) {
	return "ok", nil
}

func TestInterceptorAllowsWithinLimit(t *testing.T) {
	l := newTestLimiter(t)
	interceptor := UnaryServerInterceptor(l, nil, Config{Limit: 2, Window: time.Minute})

	ctx := withPeer(context.Background(), "10.0.0.1")
	info := &grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}

	for i := 0; i < 2; i++ {
		if _, err := interceptor(ctx, nil, info, okHandler); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
}

func TestInterceptorRejectsOverLimit(t *testing.T) {
	l := newTestLimiter(t)
	interceptor := UnaryServerInterceptor(l, nil, Config{Limit: 1, Window: time.Minute})

	ctx := withPeer(context.Background(), "10.0.0.2")
	info := &grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}

	if _, err := interceptor(ctx, nil, info, okHandler); err != nil {
		t.Fatalf("1st request: %v", err)
	}
	_, err := interceptor(ctx, nil, info, okHandler)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", err)
	}
}

func TestInterceptorUsesPerMethodLimit(t *testing.T) {
	l := newTestLimiter(t)
	limits := MethodLimits{
		"/some.Service/Login": {Limit: 1, Window: time.Minute},
	}
	interceptor := UnaryServerInterceptor(l, limits, Config{Limit: 100, Window: time.Minute})

	ctx := withPeer(context.Background(), "10.0.0.3")
	loginInfo := &grpc.UnaryServerInfo{FullMethod: "/some.Service/Login"}
	otherInfo := &grpc.UnaryServerInfo{FullMethod: "/some.Service/Other"}

	if _, err := interceptor(ctx, nil, loginInfo, okHandler); err != nil {
		t.Fatalf("1st login: %v", err)
	}
	if _, err := interceptor(ctx, nil, loginInfo, okHandler); status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("expected Login's tighter limit to reject the 2nd call, got %v", err)
	}
	// Same client, different method — must not share Login's tight budget.
	if _, err := interceptor(ctx, nil, otherInfo, okHandler); err != nil {
		t.Fatalf("expected Other to use the generous default limit, got %v", err)
	}
}

func TestInterceptorSkipsExemptMethods(t *testing.T) {
	l := newTestLimiter(t)
	interceptor := UnaryServerInterceptor(l, nil, Config{Limit: 1, Window: time.Minute}, "/grpc.health.v1.Health/Check")

	ctx := withPeer(context.Background(), "10.0.0.4")
	info := &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}

	for i := 0; i < 5; i++ {
		if _, err := interceptor(ctx, nil, info, okHandler); err != nil {
			t.Fatalf("exempt method should never be rate-limited, call %d: %v", i+1, err)
		}
	}
}

func TestInterceptorTracksClientsIndependently(t *testing.T) {
	l := newTestLimiter(t)
	interceptor := UnaryServerInterceptor(l, nil, Config{Limit: 1, Window: time.Minute})
	info := &grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}

	ctxA := withPeer(context.Background(), "10.0.0.5")
	ctxB := withPeer(context.Background(), "10.0.0.6")

	if _, err := interceptor(ctxA, nil, info, okHandler); err != nil {
		t.Fatalf("client A 1st call: %v", err)
	}
	if _, err := interceptor(ctxB, nil, info, okHandler); err != nil {
		t.Fatalf("client B should have an independent limit from client A: %v", err)
	}
}
