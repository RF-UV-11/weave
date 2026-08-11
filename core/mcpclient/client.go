// Package mcpclient implements the minimal slice of MCP that core needs to
// cache a connector's tool manifest: a single JSON-RPC "tools/list" call
// over HTTP. It is not a general MCP client — the real client (initialize,
// tools/call, stdio/SSE transports) lives in orchestrator per
// docs/architecture/ARCHITECTURE.md §3; core only ever caches the manifest.
package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolsListResult struct {
	Tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"tools"`
}

// ErrMissingDescription is returned when a connector's tools/list response
// includes one or more tools with no description. Tool descriptions are
// mandatory context (docs/architecture/ARCHITECTURE.md §3): a tool's
// description travels with its result at call time so the planner/agent
// never reasons about a tool's output without knowing what the tool claims
// to do, so a connector missing one is rejected at registration time
// rather than silently accepted.
type ErrMissingDescription struct {
	ToolNames []string
}

func (e *ErrMissingDescription) Error() string {
	return fmt.Sprintf("mcpclient: connector tools missing a description: %v", e.ToolNames)
}

// ListTools calls "tools/list" against an HTTP-transport MCP server and
// returns the raw JSON of its result, suitable for caching verbatim as a
// connector's capability_manifest. Returns *ErrMissingDescription if any
// returned tool has no description.
func ListTools(ctx context.Context, endpoint string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: struct{}{}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Response size limits on connector replies (docs/architecture/SECURITY.md §4).
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: request failed: %w", err)
	}
	defer resp.Body.Close()

	const maxManifestBytes = 1 << 20 // 1 MiB
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("mcpclient: read response: %w", err)
	}
	if len(raw) > maxManifestBytes {
		return nil, fmt.Errorf("mcpclient: response exceeds %d bytes", maxManifestBytes)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcpclient: unexpected status %d", resp.StatusCode)
	}

	var parsed rpcResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("mcpclient: decode response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("mcpclient: rpc error %d: %s", parsed.Error.Code, parsed.Error.Message)
	}

	var result toolsListResult
	if err := json.Unmarshal(parsed.Result, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: decode tools/list result: %w", err)
	}
	var missing []string
	for _, tool := range result.Tools {
		if strings.TrimSpace(tool.Description) == "" {
			missing = append(missing, tool.Name)
		}
	}
	if len(missing) > 0 {
		return nil, &ErrMissingDescription{ToolNames: missing}
	}

	return parsed.Result, nil
}
