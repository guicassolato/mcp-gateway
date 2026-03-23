##@ Vault Token Exchange Example

.PHONY: vault-token-exchange-setup
vault-token-exchange-setup: ## Setup Vault token exchange example with GitHub MCP server (requires: make local-env-setup && make auth-example-setup)
	@if [ -z "$$GITHUB_PAT" ]; then \
		echo "Error: GITHUB_PAT environment variable is not set"; \
		echo ""; \
		echo "Get a token at: https://github.com/settings/tokens/new"; \
		echo "Required permissions: read:user"; \
		echo ""; \
		echo "  export GITHUB_PAT=\"ghp_YOUR_GITHUB_TOKEN_HERE\""; \
		echo ""; \
		exit 1; \
	fi
	@echo "========================================="
	@echo "Setting up Vault Token Exchange Example"
	@echo "========================================="
	@echo ""
	@echo "Using GITHUB_PAT from environment ($${GITHUB_PAT:0:4}...)"
	@echo ""
	@echo "Step 1/5: Creating GitHub MCP server resources..."
	@kubectl apply -f config/samples/remote-github/serviceentry.yaml
	@kubectl apply -f config/samples/remote-github/destinationrule.yaml
	@kubectl apply -f config/samples/remote-github/httproute.yaml
	@printf 'apiVersion: v1\nkind: Secret\nmetadata:\n  name: github-token\n  namespace: mcp-test\n  labels:\n    mcp.kuadrant.io/secret: "true"\ntype: Opaque\nstringData:\n  token: "%s"\n' "$$GITHUB_PAT" | kubectl apply -f -
	@kubectl apply -f config/samples/remote-github/mcpserverregistration.yaml
	@kubectl patch httproute github-mcp-external -n mcp-test --type='json' \
		-p='[{"op":"add","path":"/spec/parentRefs/0/sectionName","value":"mcps"}]' 2>/dev/null || true
	@echo ""
	@echo "Step 2/5: Configuring Vault JWT auth..."
	@kubectl exec -n vault deploy/vault -- sh -c \
		'VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root vault auth enable jwt' 2>/dev/null || true
	@kubectl exec -n vault deploy/vault -- sh -c \
		'VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root vault write auth/jwt/config jwks_url="http://keycloak.keycloak.svc.cluster.local/realms/mcp/protocol/openid-connect/certs" default_role="authorino"'
	@echo ""
	@echo "Step 3/5: Creating Vault policy and role..."
	@printf 'path "secret/data/mcp-gateway/*" {\n  capabilities = ["read"]\n}\n' | \
		kubectl exec -i -n vault deploy/vault -- sh -c 'VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root vault policy write authorino -'
	@printf '{"role_type":"jwt","bound_audiences":["mcp-gateway"],"user_claim":"sub","policies":["authorino"],"ttl":"1h"}\n' | \
		kubectl exec -i -n vault deploy/vault -- sh -c 'VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root vault write auth/jwt/role/authorino -'
	@echo ""
	@echo "Step 4/5: Storing GitHub PAT in Vault for test user..."
	@KEYCLOAK=https://keycloak.127-0-0-1.sslip.io:8002; \
	ADMIN_TOKEN=$$(curl -sk $$KEYCLOAK/realms/master/protocol/openid-connect/token \
		-d 'grant_type=password' -d 'client_id=admin-cli' \
		-d 'username=admin' -d 'password=admin' | jq -r .access_token); \
	USER_SUB=$$(curl -sk $$KEYCLOAK/admin/realms/mcp/users \
		-H "Authorization: Bearer $$ADMIN_TOKEN" | jq -r '.[] | select(.username=="mcp") | .id'); \
	echo "User sub: $$USER_SUB"; \
	kubectl exec -n vault deploy/vault -- sh -c \
		"VAULT_ADDR=http://127.0.0.1:8200 VAULT_TOKEN=root vault kv put secret/mcp-gateway/$$USER_SUB github_pat=$$GITHUB_PAT"
	@echo ""
	@echo "Step 5/5: Adding GitHub tool roles to Keycloak..."
	@KEYCLOAK=https://keycloak.127-0-0-1.sslip.io:8002; \
	ADMIN_TOKEN=$$(curl -sk $$KEYCLOAK/realms/master/protocol/openid-connect/token \
		-d 'grant_type=password' -d 'client_id=admin-cli' \
		-d 'username=admin' -d 'password=admin' | jq -r .access_token); \
	curl -sk -X POST "$$KEYCLOAK/admin/realms/mcp/clients" \
		-H "Authorization: Bearer $$ADMIN_TOKEN" \
		-H "Content-Type: application/json" \
		-d '{"clientId":"mcp-test/github","publicClient":true,"standardFlowEnabled":false,"fullScopeAllowed":false}'; \
	CLIENT_UUID=$$(curl -sk "$$KEYCLOAK/admin/realms/mcp/clients?clientId=mcp-test/github" \
		-H "Authorization: Bearer $$ADMIN_TOKEN" | jq -r '.[0].id'); \
	curl -sk -X POST "$$KEYCLOAK/admin/realms/mcp/clients/$$CLIENT_UUID/roles" \
		-H "Authorization: Bearer $$ADMIN_TOKEN" \
		-H "Content-Type: application/json" \
		-d '{"name":"get_me"}'; \
	GROUP_UUID=$$(curl -sk "$$KEYCLOAK/admin/realms/mcp/groups" \
		-H "Authorization: Bearer $$ADMIN_TOKEN" | jq -r '.[] | select(.name=="accounting") | .id'); \
	ROLE=$$(curl -sk "$$KEYCLOAK/admin/realms/mcp/clients/$$CLIENT_UUID/roles/get_me" \
		-H "Authorization: Bearer $$ADMIN_TOKEN"); \
	curl -sk -X POST "$$KEYCLOAK/admin/realms/mcp/groups/$$GROUP_UUID/role-mappings/clients/$$CLIENT_UUID" \
		-H "Authorization: Bearer $$ADMIN_TOKEN" \
		-H "Content-Type: application/json" \
		-d "[$$ROLE]"
	@echo ""
	@echo "Vault token exchange setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Apply the AuthPolicy from docs/guides/vault-token-exchange.md (Step 3)"
	@echo "  2. Open MCP Inspector with 'make inspect-gateway'"
	@echo "  3. Log in as mcp/mcp and call github_get_me"
	@echo ""
