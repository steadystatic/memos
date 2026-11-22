package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/usememos/memos/internal/httphandler"
	"github.com/usememos/memos/internal/profile"
	memosproto "github.com/usememos/memos/proto"
	"github.com/usememos/memos/server/api"
)

// maxMCPRequestBytes caps the /mcp request body. It tracks the API limit because
// every tool call is forwarded in-process through the API routes. The SDK
// enforces its own 4 MiB default, so the value is passed to the transport as
// well; otherwise attachment uploads over MCP fail with 413 before the API sees them.
const maxMCPRequestBytes int64 = api.MaxAPIRequestBytes

// toolCatalogTTL is the freshness hint advertised on tools/list and
// server/discover results. The catalog is fixed at startup, so clients may cache
// it for a long time instead of re-listing on every session.
const toolCatalogTTL = 24 * time.Hour

// MCPService serves the OpenAPI-driven MCP endpoint.
type MCPService struct {
	profile *profile.Profile

	operationsByTool map[string]*registeredOperation
	handler          http.Handler
}

// NewMCPService creates an MCP service backed by the in-process API routes.
func NewMCPService(profile *profile.Profile, handler http.Handler) (*MCPService, error) {
	spec, err := loadMCPServiceOpenAPISpec()
	if err != nil {
		return nil, err
	}
	registry, err := buildOperationRegistry(spec)
	if err != nil {
		return nil, err
	}
	tools, operationsByTool, err := buildCuratedTools(registry)
	if err != nil {
		return nil, err
	}

	version := "dev"
	if profile != nil && profile.Version != "" {
		version = profile.Version
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "memos",
		Version: version,
	}, &sdkmcp.ServerOptions{
		// The tool catalog never changes after startup, so do not advertise
		// listChanged: new-protocol clients would otherwise hold a
		// subscriptions/listen stream open for notifications that never come.
		// Leaving Logging unset also drops the deprecated logging capability.
		Capabilities: &sdkmcp.ServerCapabilities{
			Tools: &sdkmcp.ToolCapabilities{},
		},
	})
	server.AddReceivingMiddleware(catalogCacheMiddleware)

	adapter := newAPIAdapter(handler)
	for _, tool := range tools {
		operation := operationsByTool[tool.Name]
		server.AddTool(tool, newMCPToolHandler(adapter, operation))
	}

	streamableHandler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server {
		return server
	}, &sdkmcp.StreamableHTTPOptions{
		// Stateless is also what lets the SDK serve protocol version
		// 2026-07-28; stateful handlers negotiate down to 2025-11-25.
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: maxMCPRequestBytes,
		// memos is typically served behind a reverse proxy with the app bound to a
		// loopback address while the public Host header is a real domain. The SDK's
		// DNS-rebinding guard treats that shape as an attack and rejects every
		// request with 403 ("invalid Host header"). Disable it and rely on memos'
		// own Origin/Host allowlist (see RegisterRoutes -> isAllowedMCPOrigin) for
		// CSRF / DNS-rebinding protection instead.
		DisableLocalhostProtection: true,
	})
	return &MCPService{
		profile:          profile,
		operationsByTool: operationsByTool,
		handler:          streamableHandler,
	}, nil
}

// catalogCacheMiddleware stamps the static-catalog TTL onto results the SDK
// would otherwise mark immediately stale (ttlMs: 0).
func catalogCacheMiddleware(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, request sdkmcp.Request) (sdkmcp.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil {
			return result, err
		}
		switch typed := result.(type) {
		case *sdkmcp.ListToolsResult:
			typed.TTLMs = int(toolCatalogTTL.Milliseconds())
		case *sdkmcp.DiscoverResult:
			typed.TTLMs = int(toolCatalogTTL.Milliseconds())
		default:
			// Other results carry no catalog TTL.
		}
		return result, nil
	}
}

func loadMCPServiceOpenAPISpec() (*openAPISpec, error) {
	spec := &openAPISpec{}
	if err := yaml.Unmarshal(memosproto.OpenAPIYAML(), spec); err != nil {
		return nil, errors.Wrap(err, "failed to parse embedded OpenAPI spec")
	}
	if spec.Paths == nil {
		return nil, errors.New("embedded OpenAPI spec has no paths")
	}
	return spec, nil
}

func newMCPToolHandler(adapter *apiAdapter, operation *registeredOperation) sdkmcp.ToolHandler {
	return func(ctx context.Context, request *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		arguments := map[string]any{}
		if request.Params != nil && len(request.Params.Arguments) > 0 {
			if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
				return newToolErrorResult(errors.Wrap(err, "failed to decode MCP tool arguments").Error()), nil
			}
		}
		if err := validateToolArguments(operation.InputSchema, arguments); err != nil {
			return newToolErrorResult(err.Error()), nil
		}

		authorization := ""
		if request.Extra != nil {
			authorization = request.Extra.Header.Get("Authorization")
		}
		return adapter.execute(ctx, operation.Operation, arguments, authorization)
	}
}

// maxMCPControlPeekBytes bounds how many bytes RegisterRoutes inspects to handle
// stateless client notifications (e.g. notifications/roots/list_changed) and ping
// without buffering large tool payloads such as attachment uploads.
const maxMCPControlPeekBytes = 4096

type mcpControlEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
}

type readCloser struct {
	io.Reader
	io.Closer
}

// normalizeMCPAccept ensures POST /mcp carries both media types required by go-sdk's
// StreamableHTTPHandler when a JSON-only client omits text/event-stream (safe because
// memos runs with JSONResponse: true and never streams SSE).
func normalizeMCPAccept(r *http.Request) {
	if r.Method != http.MethodPost {
		return
	}
	values := r.Header.Values("Accept")
	if len(values) == 0 {
		r.Header.Set("Accept", "application/json, text/event-stream")
		return
	}
	joined := strings.ToLower(strings.Join(values, ","))
	if strings.Contains(joined, "*/*") {
		return
	}
	if strings.Contains(joined, "application/json") && !strings.Contains(joined, "text/event-stream") {
		r.Header.Add("Accept", "text/event-stream")
	} else if strings.TrimSpace(joined) == "" {
		r.Header.Set("Accept", "application/json, text/event-stream")
	}
}

// handleStatelessControlMessage short-circuits client notifications (notifications/*)
// and ping requests that memos's stateless, tools-only MCP server always accepts as
// no-ops. This avoids 400 Bad Request failures when MCP clients send legacy/background
// notifications (such as notifications/roots/list_changed) or ping on a 2026-07-28
// session without _meta or Mcp-Method headers.
func handleStatelessControlMessage(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost || r.Body == nil || r.ContentLength > maxMCPControlPeekBytes {
		return false
	}

	peek, err := io.ReadAll(io.LimitReader(r.Body, maxMCPControlPeekBytes+1))
	if err != nil || len(peek) > maxMCPControlPeekBytes {
		r.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(peek), r.Body), Closer: r.Body}
		return false
	}

	var env mcpControlEnvelope
	if json.Unmarshal(peek, &env) == nil && env.JSONRPC == "2.0" {
		idTrimmed := bytes.TrimSpace(env.ID)
		hasID := len(idTrimmed) > 0 && !bytes.Equal(idTrimmed, []byte("null"))
		if !hasID && strings.HasPrefix(env.Method, "notifications/") {
			w.WriteHeader(http.StatusAccepted)
			return true
		}
		if hasID && env.Method == "ping" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":`))
			_, _ = w.Write(idTrimmed)
			_, _ = w.Write([]byte(`,"result":{}}`))
			return true
		}
	}

	r.Body = readCloser{Reader: bytes.NewReader(peek), Closer: r.Body}
	return false
}

// RegisterRoutes registers the streamable HTTP MCP endpoint.
func (s *MCPService) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxMCPRequestBytes {
			httphandler.WriteError(w, r, &httphandler.Error{Code: http.StatusRequestEntityTooLarge, Message: http.StatusText(http.StatusRequestEntityTooLarge)})
			return
		}
		if !isAllowedMCPOrigin(r.Host, r.Header.Get("Origin"), s.profile) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxMCPRequestBytes)
		normalizeMCPAccept(r)
		if handleStatelessControlMessage(w, r) {
			return
		}
		s.handler.ServeHTTP(w, r)
	})
}
