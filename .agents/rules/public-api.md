# Public API

Prefix: API. This is a library. Every exported identifier is something a caller may already depend on.

- **API1:** Do not rename, remove or change the signature of an exported identifier without a CHANGELOG line that starts with `Breaking:` and names the replacement, as the v1.15.0 entry does for the removed `echo` packages. Prefer adding a new identifier and marking the old one `// Deprecated: Use X instead.`, as `GetListenerAddr` does in `serbewr/server.go`.
- **API2:** New knobs are functional options named `With<Thing>` that return the package's `<Type>Option`, applied over a config struct whose defaults come from the root package (`BasicAuth` in `serbewr/middleware/basic_auth.go`, `New` in `klhayyent/client.go`). Middleware option names carry the middleware name when they could collide in the `middleware` package (`WithBasicAuthRealm`, `WithBearerAuthTokens`, `WithRecoveryStatusCode`).
- **API3:** Default values live in `defaults.go` as `Default*` constants and are read from there by every package. Do not hardcode a timeout, size or header value in a package when a root constant exists or should exist.
- **API4:** Header names, content types, schemes, network types, context keys and log field names are root constants (`headers.go`, `content_types.go`, `network_types.go`, `context_keys.go`, `log_fields.go`). Use them instead of string literals, and add one there before using a new header anywhere.
- **API5:** The `HTTP_SERVER_*` env var names are part of the API. A new server setting gets a `Config` field with an `env` tag in `serbewr/config.go`, an `EnvVarName*` constant in `env_var_names.go`, a `Default*` constant, an entry in the `parseConfig` defaults map, and a row in the configuration table of `docs/server.md`.
- **API6:** A new package keeps the phonetic naming (`serbewr`, `prawxxey`, `dabluvee-es`, `klhayyent`) and gets a page under `docs/` plus a row in the README documentation table in the same change.
- **API7:** Keep the root package free of server state and imports of the subpackages. Subpackages import the root, never the reverse.
