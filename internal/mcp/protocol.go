package mcp

import "encoding/json"

const (
	// modernProtocolVersion declares its version on every request.
	modernProtocolVersion = "2026-07-28"
	// latestLegacyProtocolVersion is the newest handshake-based revision the
	// server answers when a legacy client asks for one it does not know.
	latestLegacyProtocolVersion = "2025-11-25"
	// defaultProtocolVersion serves a legacy client that never initializes.
	defaultProtocolVersion = "2025-03-26"

	// channelFeynman names the Feynman HTTP channel in proposal sources and
	// audit records; channelExternal names the local socket.
	channelFeynman  = "mcp"
	channelExternal = "external-ipc"

	errCodeParseError       = -32700
	errCodeInvalidRequest   = -32600
	errCodeMethodNotFound   = -32601
	errCodeInvalidParams    = -32602
	errCodeInternalError    = -32603
	errCodeResourceNotFound = -32002
	// errCodeUnsupportedProtocolVersion reports a request that states a
	// protocol revision this server does not implement.
	errCodeUnsupportedProtocolVersion = -32022
)

var supportedModernProtocolVersions = []string{modernProtocolVersion}

var supportedProtocolVersions = map[string]bool{
	latestLegacyProtocolVersion: true,
	"2025-06-18":                true,
	defaultProtocolVersion:      true,
	"2024-11-05":                true,
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type mcpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type mcpResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type mcpTextResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text"`
}

type mcpTool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
}

type mcpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallToolResult struct {
	ResultType        string           `json:"resultType,omitempty"`
	Meta              map[string]any   `json:"_meta,omitempty"`
	Content           []mcpTextContent `json:"content"`
	StructuredContent any              `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError,omitempty"`
}
