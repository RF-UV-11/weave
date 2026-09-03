package netguard

import (
	"context"
	"net"
	"testing"
)

type fakeResolver map[string][]net.IP

func (f fakeResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	ips, ok := f[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return ips, nil
}

func TestValidatePublicURLAcceptsPublicAddress(t *testing.T) {
	resolver := fakeResolver{"api.example.com": {net.ParseIP("93.184.216.34")}} // a real public IP block (example.com)
	if err := ValidatePublicURL(context.Background(), resolver, "https://api.example.com/v1/orders", false); err != nil {
		t.Fatalf("expected a public address to be accepted, got %v", err)
	}
}

func TestValidatePublicURLRejectsCloudMetadataIP(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "http://169.254.169.254/latest/meta-data/", false); err == nil {
		t.Fatal("expected the cloud metadata address to be rejected")
	}
}

func TestValidatePublicURLRejectsLoopback(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "http://127.0.0.1:9090/", false); err == nil {
		t.Fatal("expected loopback to be rejected")
	}
}

func TestValidatePublicURLRejectsLocalhostHostname(t *testing.T) {
	resolver := fakeResolver{"localhost": {net.ParseIP("127.0.0.1")}}
	if err := ValidatePublicURL(context.Background(), resolver, "http://localhost:9090/", false); err == nil {
		t.Fatal("expected a hostname resolving to loopback to be rejected")
	}
}

func TestValidatePublicURLRejectsPrivateRFC1918(t *testing.T) {
	cases := []string{"http://10.0.0.5/", "http://172.16.0.5/", "http://192.168.1.5/"}
	for _, u := range cases {
		if err := ValidatePublicURL(context.Background(), fakeResolver{}, u, false); err == nil {
			t.Fatalf("expected %s to be rejected as private", u)
		}
	}
}

func TestValidatePublicURLRejectsHostnameResolvingToPrivateIP(t *testing.T) {
	// The classic SSRF-via-DNS-rebinding-adjacent case: a public-looking
	// hostname that actually resolves internally.
	resolver := fakeResolver{"internal-service.attacker.example": {net.ParseIP("10.1.2.3")}}
	if err := ValidatePublicURL(context.Background(), resolver, "http://internal-service.attacker.example/", false); err == nil {
		t.Fatal("expected a hostname resolving to a private IP to be rejected")
	}
}

func TestValidatePublicURLRejectsNonHTTPScheme(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "file:///etc/passwd", false); err == nil {
		t.Fatal("expected a non-http(s) scheme to be rejected")
	}
}

func TestValidatePublicURLRejectsUnresolvableHost(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "http://does-not-exist.invalid/", false); err == nil {
		t.Fatal("expected an unresolvable host to be rejected")
	}
}

func TestValidatePublicURLRejectsMalformedURL(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "://not-a-url", false); err == nil {
		t.Fatal("expected a malformed URL to be rejected")
	}
}

func TestValidatePublicURLAllowPrivateBypassesBlocklist(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "http://127.0.0.1:8765/mcp", true); err != nil {
		t.Fatalf("expected allowPrivate=true to accept a loopback address, got %v", err)
	}
}

func TestValidatePublicURLAllowPrivateStillRejectsBadScheme(t *testing.T) {
	// allowPrivate skips the IP-range blocklist only — basic URL sanity
	// checks always apply, even in local-dev mode.
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "file:///etc/passwd", true); err == nil {
		t.Fatal("expected a non-http(s) scheme to be rejected even with allowPrivate=true")
	}
}

func TestValidatePublicURLAllowPrivateStillRejectsMalformedURL(t *testing.T) {
	if err := ValidatePublicURL(context.Background(), fakeResolver{}, "://not-a-url", true); err == nil {
		t.Fatal("expected a malformed URL to be rejected even with allowPrivate=true")
	}
}
