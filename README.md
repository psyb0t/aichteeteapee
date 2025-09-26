# aichteeteapee

> **Pronounced "HTTP"** - because sometimes the best fucking code comes with wordplay.

**aichteeteapee** is a batteries-included HTTP utilities library that gets you from `go mod init` to production-ready server in under 10 lines of code. Built on the philosophy of sane defaults, zero boilerplate, and easy customization.

Perfect for:
- 🚀 **Rapid prototyping** with production-ready foundations
- 🏗️ **Microservices** that need HTTP + WebSocket capabilities
- 📡 **APIs** requiring file uploads, static serving, and real-time features
- 🛠️ **Any Go project** that wants HTTP functionality without the fucking boilerplate

## Quick Start - Zero to Hero

```go
package main

import (
    "context"
    "log"

    "github.com/psyb0t/aichteeteapee/server"
    "github.com/psyb0t/aichteeteapee/server/middleware"
    "github.com/psyb0t/aichteeteapee/server/dabluvee-es/wshub"
)

func main() {
    // Create server with sane defaults
    srv, err := server.New()
    if err != nil {
        log.Fatal(err)
    }

    // Create WebSocket hub
    myWebSocketHub := wshub.NewHub("main")

    // Build your routes with batteries-included middleware
    router := &server.Router{
        // Global middleware applied to all routes
        GlobalMiddlewares: []middleware.Middleware{
            middleware.CORS(),           // Smart CORS handling
            middleware.SecurityHeaders(), // Production security headers
            middleware.Logger(),         // Structured request logging
            middleware.Recovery(),       // Panic recovery
        },

        // Static file serving
        Static: []server.StaticRouteConfig{
            {
                Dir:                   "./web/static",
                Path:                  "/static/",
                DirectoryIndexingType: server.DirectoryIndexingTypeJSON, // Smart directory listing
            },
        },

        // Route groups
        Groups: []server.GroupConfig{
            {
                Path: "/api",
                Routes: []server.RouteConfig{
                    {Method: "GET", Path: "/health", Handler: srv.HealthHandler},
                    {Method: "POST", Path: "/echo", Handler: srv.EchoHandler},
                    {Method: "POST", Path: "/upload", Handler: srv.FileUploadHandler("./uploads")},
                },
            },
            {
                Path: "/ws",
                Routes: []server.RouteConfig{
                    {Method: "GET", Path: "", Handler: wshub.UpgradeHandler(myWebSocketHub)},
                },
            },
        },
    }

    // Start server - HTTP on :8080, HTTPS on :8443, graceful shutdown
    log.Fatal(srv.Start(context.Background(), router))
}
```

That's it! You now have:
- ✅ HTTP + HTTPS servers running
- ✅ CORS, security headers, request logging
- ✅ File uploads with processing hooks & filename options
- ✅ Static file serving with directory browsing (HTML/JSON)
- ✅ WebSocket hub for real-time features with event metadata
- ✅ Request ID generation & extraction utilities
- ✅ Content-type enforcement middleware
- ✅ Timeout middleware with presets (short/default/long)
- ✅ Granular security header control
- ✅ Health checks and echo endpoints
- ✅ Unix socket bridge for external tool integration
- ✅ Graceful shutdown handling

## Complete Example - Full-Featured App

Here's a more comprehensive example showing advanced features:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/psyb0t/aichteeteapee/server"
    "github.com/psyb0t/aichteeteapee/server/middleware"
    dabluveees "github.com/psyb0t/aichteeteapee/server/dabluvee-es"
    "github.com/psyb0t/aichteeteapee/server/dabluvee-es/wshub"
)

// WebSocket hubs
var chatHub = wshub.NewHub("chat")
var notificationHub = wshub.NewHub("notifications")

func main() {
    // Initialize WebSocket event handlers
    chatHub.RegisterEventHandlers(map[dabluveees.EventType]wshub.EventHandler{
        "chat.message": func(hub wshub.Hub, client *wshub.Client, event *dabluveees.Event) error {
            hub.BroadcastToAll(event)
            return nil
        },
    })

    // Server with custom config
    srv, err := server.NewWithConfig(server.Config{
        ListenAddress:       "127.0.0.1:3000",
        TLSListenAddress:    "127.0.0.1:3443",
        ServiceName:         "MyAwesomeAPI",
        ReadTimeout:         30 * time.Second,
        WriteTimeout:        30 * time.Second,
        FileUploadMaxMemory: 50 << 20, // 50MB
    })
    if err != nil {
        log.Fatal(err)
    }

    // Advanced routing with nested groups and middleware
    router := &server.Router{
        GlobalMiddlewares: []middleware.Middleware{
            middleware.RequestID(),                    // Request tracking
            middleware.Logger(),                       // Structured logging
            middleware.CORS(     // Custom CORS
                middleware.WithAllowedOrigins("https://myapp.com"),
                middleware.WithAllowedMethods("GET", "POST", "PUT", "DELETE"),
                middleware.WithAllowCredentials(true),
            ),
            middleware.SecurityHeaders(
                middleware.WithContentSecurityPolicy("default-src 'self'"),
                middleware.WithStrictTransportSecurity("max-age=31536000; includeSubDomains"),
            ),
            middleware.Recovery(),                     // Panic recovery
        },

        // Multiple static routes with different configs
        Static: []server.StaticRouteConfig{
            {
                Dir:                   "./web/assets",
                Path:                  "/assets/",
                DirectoryIndexingType: server.DirectoryIndexingTypeNone, // No directory listing for assets
            },
            {
                Dir:                   "./public",
                Path:                  "/files/",
                DirectoryIndexingType: server.DirectoryIndexingTypeHTML, // Directory browsing enabled
            },
        },

        // Route groups with inherited middleware
        Groups: []server.GroupConfig{
            {
                Path: "", // Root level routes
                Routes: []server.RouteConfig{
                    {Method: "GET", Path: "/", Handler: homeHandler},
                    {Method: "GET", Path: "/health", Handler: srv.HealthHandler},
                    {Method: "POST", Path: "/upload", Handler: srv.FileUploadHandler("./uploads",
                        server.WithFileUploadHandlerPostprocessor(processUploadedFile))},
                },
            },
            {
                Path: "/api/v1",
                Middlewares: []middleware.Middleware{
                    middleware.Timeout(10 * time.Second), // API timeouts
                },
                Routes: []server.RouteConfig{
                    {Method: "GET", Path: "/users", Handler: getUsersHandler},
                    {Method: "POST", Path: "/users", Handler: createUserHandler},
                    {Method: "GET", Path: "/users/{id}", Handler: getUserHandler},
                },
                Groups: []server.GroupConfig{
                    {
                        Path: "/admin",
                        Middlewares: []middleware.Middleware{
                            middleware.BasicAuth(
                                middleware.WithBasicAuthUsers(map[string]string{"admin": "secret"}),
                                middleware.WithBasicAuthRealm("Admin Area"),
                            ),
                        },
                        Routes: []server.RouteConfig{
                            {Method: "GET", Path: "/stats", Handler: adminStatsHandler},
                            {Method: "DELETE", Path: "/users/{id}", Handler: deleteUserHandler},
                        },
                    },
                },
            },
            {
                Path: "/ws",
                Routes: []server.RouteConfig{
                    {Method: "GET", Path: "/chat", Handler: wshub.UpgradeHandler(chatHub)},
                    {Method: "GET", Path: "/notifications", Handler: wshub.UpgradeHandler(notificationHub)},
                },
            },
        },
    }

    log.Printf("🚀 Starting MyAwesome API server...")
    log.Printf("📍 HTTP:  http://127.0.0.1:3000")
    log.Printf("📍 HTTPS: https://127.0.0.1:3443")
    log.Printf("💬 Chat WebSocket: ws://127.0.0.1:3000/ws/chat")

    log.Fatal(srv.Start(context.Background(), router))
}

// Custom handlers with proper error handling
func homeHandler(w http.ResponseWriter, r *http.Request) {
    response := map[string]string{
        "message": "Welcome to MyAwesome API!",
        "version": "1.0.0",
        "docs":    "/api/docs",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
    // Your business logic here
    users := []map[string]interface{}{
        {"id": 1, "name": "Alice", "email": "alice@example.com"},
        {"id": 2, "name": "Bob", "email": "bob@example.com"},
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "user created"})
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"user": "details"})
}

func adminStatsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"stats": "admin data"})
}

func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{"status": "user deleted"})
}

func processUploadedFile(filepath string, filename string) error {
    log.Printf("Processing uploaded file: %s -> %s", filename, filepath)
    // Your custom file processing logic
    return nil
}

// WebSocket hub for real-time chat
var chatHub = wshub.NewHub("chat")
var notificationHub = wshub.NewHub("notifications")

func init() {
    chatHub.RegisterEventHandlers(map[dabluveees.EventType]wshub.EventHandler{
        "chat.message": func(hub wshub.Hub, client *wshub.Client, event *dabluveees.Event) error {
            // Broadcast message to all connected clients
            hub.BroadcastToAll(event)
            return nil
        },
        "chat.join": func(hub wshub.Hub, client *wshub.Client, event *dabluveees.Event) error {
            log.Printf("User joined chat: %s", client.ID())
            hub.BroadcastToAll(event)
            return nil
        },
    })
}
```

## Key Features

### 🎯 **Zero-Config Production Ready**
- **Secure defaults**: HTTPS, security headers, CORS, timeouts
- **Graceful shutdown**: Proper resource cleanup and connection draining
- **Built-in monitoring**: Health checks, request logging, panic recovery
- **File caching**: DoS protection with intelligent cache management

### 🌐 **Advanced HTTP Server**
- **Dual listeners**: HTTP and HTTPS running concurrently
- **Flexible routing**: Nested groups with middleware inheritance
- **Static files**: Smart directory indexing (HTML/JSON), SPA support
- **File uploads**: Configurable processing, size limits, secure paths

### ⚡ **WebSocket Systems**

**Hub System** (for real-time apps):
- **Event-driven**: Type-safe JSON events with metadata
- **Multi-client**: Hub → Client → Connection architecture
- **Broadcasting**: All clients, specific clients, or subscribers
- **Thread-safe**: Proper concurrency with atomic operations

**Unix Socket Bridge** (for external tool integration):
- **Bidirectional bridge**: WebSocket ↔ Unix domain sockets
- **External tool integration**: Shell scripts, CLI tools, other processes
- **File-based communication**: Simple read/write operations
- **Tool chaining**: Connect WebSocket apps to existing Unix toolchain

### 🔐 **Security First**
- **Path traversal protection**: Multiple validation layers
- **Security headers**: HSTS, CSP, X-Frame-Options, and more
- **CORS**: Smart origin validation with preflight optimization
- **Basic Auth**: Timing attack protection, configurable realms
- **Request limits**: Timeouts, body size limits, rate limiting ready

### 🛠️ **Developer Experience**
- **Sane defaults**: Works out of the box, customize when needed
- **Structured logging**: Consistent field names, request tracing
- **Request ID utilities**: Automatic generation and extraction helpers
- **Content-type enforcement**: API protection with configurable types
- **Timeout presets**: Short (5s), Default (30s), Long (5m) options
- **Granular security headers**: Enable/disable individual headers
- **File upload options**: UUID/DateTime/None filename prepending
- **Event metadata system**: Thread-safe WebSocket event enrichment
- **Error handling**: Proper HTTP status codes and JSON responses
- **Middleware**: Composable, reusable, with proper ordering

## Advanced Usage

### Custom Middleware

```go
// Create your own middleware
func AuthMiddleware(secret string) middleware.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := r.Header.Get("Authorization")
            if !validateToken(token, secret) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// Use in route groups
{
    Path: "/api/protected",
    Middlewares: []middleware.Middleware{
        AuthMiddleware("your-secret-key"),
    },
    Routes: []server.RouteConfig{
        {Method: "GET", Path: "/profile", Handler: profileHandler},
    },
}
```

### WebSocket Events

```go
// Define your event types
const (
    EventTypeChatMessage dabluveees.EventType = "chat.message"
    EventTypeUserJoin    dabluveees.EventType = "user.join"
    EventTypeUserLeave   dabluveees.EventType = "user.leave"
)

// Create event handlers
chatHub := wshub.NewHub("chat")
chatHub.RegisterEventHandlers(map[dabluveees.EventType]wshub.EventHandler{
    EventTypeChatMessage: func(hub wshub.Hub, client *wshub.Client, event *dabluveees.Event) error {
        // Parse message data
        var messageData struct {
            Text   string `json:"text"`
            UserID string `json:"userId"`
        }
        if err := json.Unmarshal(event.Data, &messageData); err != nil {
            return err
        }

        // Add timestamp and broadcast
        event.Metadata.Set("timestamp", time.Now().Unix())
        hub.BroadcastToAll(event)
        return nil
    },
})
```

### Unix Socket Bridge - External Tool Integration

```go
// Unix socket bridge for external tool integration
import "github.com/psyb0t/aichteeteapee/server/dabluvee-es/wsunixbridge"

// Create Unix bridge handler
bridgeHandler := wsunixbridge.NewUpgradeHandler(
    "./sockets",  // Directory for Unix socket files
    func(connection *wsunixbridge.Connection) error {
        log.Printf("Unix bridge connection established: %s", connection.ID)
        log.Printf("Writer socket: %s", connection.WriterUnixSock.Path)
        log.Printf("Reader socket: %s", connection.ReaderUnixSock.Path)
        return nil
    },
)

// Add to routes
{Method: "GET", Path: "/unixsock", Handler: bridgeHandler}

// External tools can now:
// 1. Connect to writer socket to receive WebSocket data
// 2. Write to reader socket to send data to WebSocket
//
// Example: echo "Hello from external tool" | socat - UNIX-CONNECT:./sockets/connection-id_input
```

### File Upload Processing

// Advanced file upload configuration
uploadHandler := srv.FileUploadHandler("./uploads",
    // Custom postprocessor for file processing
    server.WithFileUploadHandlerPostprocessor(func(
        response map[string]any,
        request *http.Request,
    ) (map[string]any, error) {
        // Add custom metadata to upload response
        response["processed_at"] = time.Now().Unix()
        response["user_ip"] = request.RemoteAddr
        return response, nil
    }),

    // Filename prepending options
    server.WithFilenamePrependType(server.FilenamePrependTypeDateTime), // Y_M_D_H_I_S_
    // Alternative: server.FilenamePrependTypeUUID (default)
    // Alternative: server.FilenamePrependTypeNone
)
```

### Advanced Middleware Features

```go
import (
    "github.com/psyb0t/aichteeteapee/server/middleware"
    "github.com/sirupsen/logrus"
)

// Request ID middleware with utility functions
router := server.Router{
    Middlewares: []middleware.Middleware{
        middleware.RequestID(), // Automatic request ID generation
    },
    Groups: []server.GroupConfig{
        {
            Path: "/api",
            Routes: []server.RouteConfig{
                {
                    Method: "GET",
                    Path: "/status",
                    Handler: func(w http.ResponseWriter, r *http.Request) {
                        // Extract request ID from context
                        reqID := middleware.GetRequestID(r)

                        response := map[string]string{
                            "status":     "ok",
                            "request_id": reqID,
                        }
                        json.NewEncoder(w).Encode(response)
                    },
                },
            },
        },
    },
}

// Content-Type enforcement for APIs
apiGroup := server.GroupConfig{
    Path: "/api",
    Middlewares: []middleware.Middleware{
        // Only allow JSON requests
        middleware.EnforceRequestContentType("application/json"),
        // Alternative: allow multiple types
        // middleware.EnforceRequestContentType("application/json", "application/xml"),
        // Shortcut for JSON-only APIs
        // middleware.EnforceRequestContentTypeJSON(),
    },
    Routes: []server.RouteConfig{
        {Method: "POST", Path: "/users", Handler: createUserHandler},
    },
}

// Timeout middleware with presets
timeoutGroup := server.GroupConfig{
    Path: "/slow-api",
    Middlewares: []middleware.Middleware{
        middleware.Timeout(
            middleware.WithLongTimeout(), // 5 minutes
            // Alternative presets:
            // middleware.WithShortTimeout(),   // 5 seconds
            // middleware.WithDefaultTimeout(), // 30 seconds
            // middleware.WithTimeout(2 * time.Minute), // Custom
        ),
    },
    Routes: []server.RouteConfig{
        {Method: "POST", Path: "/batch-process", Handler: batchProcessHandler},
    },
}

// Security headers with granular control
secureGroup := server.GroupConfig{
    Path: "/secure",
    Middlewares: []middleware.Middleware{
        middleware.SecurityHeaders(
            // Enable specific headers
            middleware.WithXContentTypeOptions("nosniff"),
            middleware.WithXFrameOptions("DENY"),
            middleware.WithStrictTransportSecurity("max-age=31536000; includeSubDomains"),

            // Disable unwanted headers
            middleware.DisableXXSSProtection(), // If you handle XSS at app level
            middleware.DisableCSP(),            // If you have custom CSP
        ),
    },
}
```

### Enhanced WebSocket Events

```go
import "github.com/psyb0t/aichteeteapee/server/dabluvee-es"

// Event creation with metadata and utility methods
hub := wshub.NewHub("notifications")

hub.RegisterEventHandler(EventTypeNewMessage, func(hub wshub.Hub, client *wshub.Client, event *dabluveees.Event) error {
    // Add metadata to events
    enrichedEvent := event.
        WithMetadata("server_id", "api-01").
        WithMetadata("processing_time", time.Now().Unix()).
        WithTimestamp(time.Now().Unix()) // Override timestamp

    // Check if event is recent (within last 60 seconds)
    if enrichedEvent.IsRecent(60) {
        // Get event time as Go time.Time
        eventTime := enrichedEvent.GetTime()
        log.Printf("Processing recent event from %v", eventTime)

        // Broadcast with enriched metadata
        hub.BroadcastToAll(&enrichedEvent)
    }

    return nil
})

// Built-in event types available
const (
    // System events
    dabluveees.EventTypeSystemLog   // "system.log"
    dabluveees.EventTypeShellExec   // "shell.exec"
    dabluveees.EventTypeEchoRequest // "echo.request"
    dabluveees.EventTypeEchoReply   // "echo.reply"
    dabluveees.EventTypeError       // "error"
)
```

## Configuration

### Environment Variables

```bash
# Server settings
HTTP_SERVER_LISTENADDRESS=127.0.0.1:8080
HTTP_SERVER_TLSLISTENADDRESS=127.0.0.1:8443
HTTP_SERVER_SERVICENAME=MyAPI

# Security
HTTP_SERVER_TLSENABLED=true
HTTP_SERVER_TLSCERTFILE=./certs/server.crt
HTTP_SERVER_TLSKEYFILE=./certs/server.key

# Timeouts (durations)
HTTP_SERVER_READTIMEOUT=30s
HTTP_SERVER_READHEADERTIMEOUT=10s
HTTP_SERVER_WRITETIMEOUT=30s
HTTP_SERVER_IDLETIMEOUT=60s
HTTP_SERVER_MAXHEADERBYTES=1048576
HTTP_SERVER_SHUTDOWNTIMEOUT=30s

# File uploads
HTTP_SERVER_FILEUPLOADMAXMEMORY=52428800  # 50MB in bytes
```

### Programmatic Config

```go
srv, err := server.NewWithConfig(server.Config{
    ListenAddress:       "0.0.0.0:8080",
    TLSListenAddress:    "0.0.0.0:8443",
    ServiceName:         "ProductionAPI",
    ReadTimeout:         30 * time.Second,
    ReadHeaderTimeout:   10 * time.Second,
    WriteTimeout:        30 * time.Second,
    IdleTimeout:         60 * time.Second,
    MaxHeaderBytes:      1 << 20, // 1MB
    ShutdownTimeout:     30 * time.Second,
    FileUploadMaxMemory: 100 << 20, // 100MB
    TLSEnabled:          true,
    TLSCertFile:         "/etc/ssl/certs/api.crt",
    TLSKeyFile:          "/etc/ssl/private/api.key",
})
```

## Security Warnings ⚠️

**READ THIS SHIT CAREFULLY** - Security is not a fucking joke:

### 🔥 **CRITICAL - Authentication & Authorization**
- **NEVER** run without authentication in production
- **ALWAYS** validate user permissions before accessing resources
- **USE** HTTPS in production - HTTP is not secure
- **IMPLEMENT** proper session management and token validation

### 🔒 **File Upload Security**
- **VALIDATE** file types and sizes - don't trust client data
- **SCAN** uploads for malware before processing
- **STORE** uploads outside web root to prevent direct access
- **LIMIT** file extensions and use whitelist, not blacklist

### 🛡️ **Path Traversal Protection**
- Library includes protection, but **ALWAYS** validate custom file paths
- **NEVER** trust user input for file system operations
- **USE** absolute paths and proper validation for file access

### 🚨 **WebSocket Security**
- **AUTHENTICATE** WebSocket connections - they bypass normal HTTP auth
- **VALIDATE** all event data - treat it as untrusted input
- **IMPLEMENT** rate limiting for WebSocket messages
- **MONITOR** connection counts to prevent DoS attacks

### 🔐 **Production Checklist**
- [ ] HTTPS configured with valid certificates
- [ ] Security headers properly configured
- [ ] Authentication middleware on protected routes
- [ ] File upload validation and scanning
- [ ] Proper error handling (don't leak internal info)
- [ ] Request logging and monitoring in place
- [ ] Rate limiting configured
- [ ] Regular security updates

**Remember**: This library provides the tools, but YOU are responsible for using them securely. Don't be the person who gets pwned because they skipped the security setup.

## License

MIT License - See [LICENSE](LICENSE) file for details.

---

**aichteeteapee** - because building HTTP servers shouldn't be a pain in the ass.

*Pronunciation guide: "ay-ch-tee-tee-pee" = HTTP. Yes, it's a terrible pun. No apologies.*