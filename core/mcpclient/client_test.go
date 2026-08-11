package mcpclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListToolsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"book_appointment","description":"Book an appointment slot"}]}}`))
	}))
	defer srv.Close()

	result, err := ListTools(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if !strings.Contains(string(result), "book_appointment") {
		t.Fatalf("expected result to contain tool name, got %s", result)
	}
}

func TestListToolsRejectsToolMissingDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"book_appointment"}]}}`))
	}))
	defer srv.Close()

	_, err := ListTools(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error when a tool has no description")
	}
	var missingDesc *ErrMissingDescription
	if !errors.As(err, &missingDesc) {
		t.Fatalf("expected *ErrMissingDescription, got %T: %v", err, err)
	}
	if len(missingDesc.ToolNames) != 1 || missingDesc.ToolNames[0] != "book_appointment" {
		t.Fatalf("expected [book_appointment], got %v", missingDesc.ToolNames)
	}
}

func TestListToolsRejectsToolWithBlankDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"book_appointment","description":"   "}]}}`))
	}))
	defer srv.Close()

	if _, err := ListTools(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error when a tool's description is only whitespace")
	}
}

func TestListToolsAcceptsMultipleFullyDescribedTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[
			{"name":"book_appointment","description":"Book an appointment slot"},
			{"name":"cancel_appointment","description":"Cancel an existing appointment"}
		]}}`))
	}))
	defer srv.Close()

	if _, err := ListTools(context.Background(), srv.URL); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
}

func TestListToolsRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`))
	}))
	defer srv.Close()

	if _, err := ListTools(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error when the connector returns a JSON-RPC error")
	}
}

func TestListToolsNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := ListTools(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error on non-200 status")
	}
}

func TestListToolsMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	if _, err := ListTools(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error on malformed JSON response")
	}
}

func TestListToolsResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"description":"`))
		big := strings.Repeat("x", (1<<20)+10)
		w.Write([]byte(big))
		w.Write([]byte(`"}]}}`))
	}))
	defer srv.Close()

	if _, err := ListTools(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error when response exceeds the size cap")
	}
}

func TestListToolsUnreachableEndpoint(t *testing.T) {
	if _, err := ListTools(context.Background(), "http://127.0.0.1:1"); err == nil {
		t.Fatal("expected error for an unreachable endpoint")
	}
}
