# MCP Router

## Request Routing

The router handles two paths based on MCP method:

### `tools/call` - routed directly to backend MCP server
- `:authority` header set to HTTPRoute hostname (URLRewrite filter handles external rewrite)
- `:path` header set to backend path from server config
- `x-mcp-method`, `x-mcp-servername`, `x-mcp-toolname` headers for routing metadata
- `x-mcp-annotation-hints` header with tool annotations when available
- `mcp-session-id` header with the remote backend session ID
- Tool name prefix stripping before forwarding to backend
- Client headers (except pseudo-headers and `mcp-session-id`) are passed through during session initialization
