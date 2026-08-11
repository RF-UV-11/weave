package tenant

import (
	"context"
	"fmt"
	"os"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
)

func TestMain(m *testing.M) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if err := mongodb.InitDatabase(uri, "weave_core_test_tenant"); err != nil {
		fmt.Printf("tenant: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}

	code := m.Run()

	if err := mongodb.Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("tenant: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func TestCreateTenantRequiresDisplayName(t *testing.T) {
	s := NewServer()
	_, err := s.CreateTenant(t.Context(), &dataaccessv1.CreateTenantRequest{TenantType: "business"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateTenantRejectsUnknownTenantType(t *testing.T) {
	s := NewServer()
	_, err := s.CreateTenant(t.Context(), &dataaccessv1.CreateTenantRequest{
		DisplayName: "Acme",
		TenantType:  "not-a-real-type",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateAndGetTenantRoundTrip(t *testing.T) {
	s := NewServer()
	created, err := s.CreateTenant(t.Context(), &dataaccessv1.CreateTenantRequest{
		DisplayName: "Roundtrip Co",
		TenantType:  "business",
	})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	got, err := s.GetTenant(t.Context(), &dataaccessv1.GetTenantRequest{Id: created.GetTenant().GetXId()})
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got.GetTenant().GetDisplayName() != "Roundtrip Co" {
		t.Fatalf("got %+v", got.GetTenant())
	}
}

func TestGetTenantRequiresID(t *testing.T) {
	s := NewServer()
	_, err := s.GetTenant(t.Context(), &dataaccessv1.GetTenantRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestGetTenantNotFound(t *testing.T) {
	s := NewServer()
	_, err := s.GetTenant(t.Context(), &dataaccessv1.GetTenantRequest{Id: "tnt_does_not_exist"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}
