// Package netguard blocks SSRF: a tenant registering a connector or HTTP
// tool supplies an arbitrary endpoint URL, and core (and, transitively,
// orchestrator and mcp-gateway) will make real outbound requests to it.
// Without this check, a malicious or compromised tenant could point a
// "connector" at internal infrastructure — a cloud metadata endpoint
// (169.254.169.254), core's own address, another service on the private
// network — and have Weave's own trusted infra make that request for
// them. Every place that accepts a tenant-supplied endpoint at
// registration time must call ValidatePublicURL first.
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/url"
)

// Resolver is satisfied by net.DefaultResolver; a seam for tests so they
// don't depend on real DNS.
type Resolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// ValidatePublicURL rejects any URL that isn't a plain http(s) URL
// resolving only to public IP addresses. Call this at registration time
// (RegisterConnector, RegisterHttpTool) — not at call time, so a bad
// endpoint is rejected once up front rather than on every tool call.
//
// allowPrivate skips the IP-range blocklist (but never the scheme/URL
// sanity checks) — for local dev only, where connectors legitimately run
// on localhost/the docker network (reference-mcp, httptest fixtures,
// dev-stub-mcp). Defaults to false in production (core/configs); tests
// and local-only deployments opt in explicitly, it's never the default.
func ValidatePublicURL(ctx context.Context, resolver Resolver, rawURL string, allowPrivate bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("netguard: invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("netguard: scheme must be http or https, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("netguard: URL has no host")
	}
	if allowPrivate {
		return nil
	}

	// A literal IP in the URL skips DNS resolution entirely.
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("netguard: %s resolves to a non-public address", host)
		}
		return nil
	}

	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("netguard: resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("netguard: %q did not resolve to any address", host)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("netguard: %s resolves to %s, a non-public address", host, ip)
		}
	}
	return nil
}

// isBlockedIP covers loopback, RFC1918/private, link-local (which
// includes the 169.254.169.254 cloud metadata address), unspecified, and
// multicast ranges — every non-public class net.IP can classify natively.
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}
