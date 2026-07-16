# Marketplace OpenAPI alignment

## Goal

Align the newly added Skill and MCP Marketplace HTTP surface with the OCTO
OpenAPI development standard and adapt the Marketplace frontend clients.

## Load-bearing behavior

- Authenticated Skill and MCP access remains Space-scoped.
- Existing singular Skill routes remain temporarily available with standard
  deprecation headers.
- Skill archive downloads remain authorization-gated before returning or
  redirecting to object storage.
- MCP secret values remain rejected and are never persisted or returned.

## Out of scope

- octo-server and unrelated octo-web modules.
- New Marketplace product features or persistence changes.
- CLI behavior changes.

## Acceptance criteria

- Marketplace success and error responses use the standard OCTO envelopes.
- Public resources, parameters, fields, pagination, and operation IDs follow
  the vendored API specification.
- `go test ./...` and `make openapi-check` pass.
- Skill and MCP frontend API clients consume the new contract without changing
  page-level domain types.
