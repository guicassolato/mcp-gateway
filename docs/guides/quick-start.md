# Quick Start Guide

Get MCP Gateway running locally in a Kind cluster.

## Prerequisites

- [Docker](https://docs.docker.com/engine/install/) or [Podman](https://podman.io/docs/installation) installed and running
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [Helm](https://helm.sh/docs/intro/install/) installed (for Istio)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed

## Quick Setup

Set the release version and run the setup script:

```bash
export MCP_GATEWAY_VERSION=0.5.1
curl -sSL https://raw.githubusercontent.com/Kuadrant/mcp-gateway/main/scripts/quick-start.sh | bash
```

The script checks prerequisites, then walks through each step interactively:

1. **Create Kind cluster** with port mapping (`localhost:7001`)
2. **Install Gateway API CRDs and Istio** as the Gateway API provider
3. **Create the Gateway** with listeners and a NodePort service
4. **Install MCP Gateway** CRDs, controller, and MCPGatewayExtension (the controller automatically deploys the broker-router)
5. **Deploy test MCP servers** and register them with the gateway

Each step describes what it will do and waits for confirmation before proceeding.

## Test with MCP Inspector

Once the script completes, you can use [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to interact with the gateway. This requires [Node.js and npm](https://nodejs.org/en/download/).

```bash
DANGEROUSLY_OMIT_AUTH=true npx @modelcontextprotocol/inspector@latest
```

Open the inspector at [http://localhost:6274](http://localhost:6274) and configure:

- **Transport**: Streamable HTTP
- **URL**: `http://mcp.127-0-0-1.sslip.io:7001/mcp`

Click **Connect**, then browse and test the available tools:

| Tool | Description |
|------|-------------|
| `test1_hi` | Simple greeting |
| `test1_time` | Current time |
| `test1_headers` | HTTP header inspection |
| `test2_hello_world` | Greeting from server 2 |
| `test2_time` | Current time from server 2 |
| `test2_headers` | HTTP headers from server 2 |

## Cleanup

```bash
kind delete cluster
```

## API Reference

- [MCPGatewayExtension](../reference/mcpgatewayextension.md)
- [MCPServerRegistration](../reference/mcpserverregistration.md)
- [MCPVirtualServer](../reference/mcpvirtualserver.md)

## Next Steps

- **[Authentication](./authentication.md)** - Configure OAuth-based security with Keycloak
- **[Authorization](./authorization.md)** - Set up fine-grained access control
- **[Virtual MCP Servers](./virtual-mcp-servers.md)** - Create focused tool collections for specific use cases
- **[External MCP Servers](./external-mcp-server.md)** - Connect to external APIs and services
