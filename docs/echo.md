# echo — Echo framework integration

For when you want [labstack/echo](https://github.com/labstack/echo) instead of `net/http`.

## echo/

```go
e, _ := echo.New("/api", swaggerYAML, middlewares)
e.Start(ctx)
```

Auto-serves OpenAPI spec at `OASPath` and Swagger UI at `SwaggerUIPath`. Config from env:

| Variable | Default |
|---|---|
| `HTTP_ECHO_LISTENADDRESS` | `0.0.0.0:8080` |
| `HTTP_ECHO_OASPATH` | — |
| `HTTP_ECHO_SWAGGERUIPATH` | — |

## echo/middleware/

OpenAPI request validation + panic recovery + Bearer token auth in one call:

```go
middlewares := echomw.CreateDefaultAPIMiddleware(spec, echomw.BearerAuth(
    func(ctx context.Context, token string) error {
        return validateToken(ctx, token)
    },
))
```

`BearerAuth` extracts the token from the `Authorization: Bearer <token>` header and passes it to your validator. Returns `401 Unauthorized` on missing or invalid tokens.

## oapi-codegen/middleware/

Wraps [oapi-codegen/echo-middleware](https://github.com/oapi-codegen/echo-middleware) with structured error responses using `aichteeteapee.ErrorResponse`.

```go
mw := oapimw.OapiValidatorMiddleware(spec)
mw := oapimw.OapiValidatorMiddlewareWithOptions(spec, opts)
```

Validation errors return a structured JSON body with the appropriate `ErrorCode` and human-readable message. Multiple validation errors are joined into a single response.
