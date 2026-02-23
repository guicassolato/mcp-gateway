# MCP Router

## Routing Headers

Router (ext_proc) properly sets routing headers:
- `:authority` header set to HTTPRoute hostname (URLRewrite filter handles external rewrite)
- `:path` header set to custom path (e.g., `/v1/special/mcp`) when specified
- `x-mcp-api-key` header for backend API keys (to avoid OAuth conflicts)
- Authorization header added with Bearer token
- Tool name prefix stripping working correctly
