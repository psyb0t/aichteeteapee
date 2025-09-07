# aichteeteapee 🌶️

_pronounced "HTTP" because comedic genius was involved here_

## dafuq is dis bish?

**aichteeteapee** is a collection of HTTP utilities that don't suck. It's got two main parts:

1. **Root package**: Common HTTP utilities you can use anywhere - JSON responses, request parsing, headers, error codes, etc.
2. **Server package**: A complete batteries-included web server with WebSocket support, middleware, static files, and all that jazz

Use just the utilities with your existing server, or go full beast mode with the complete server. Your call.

## Installation

```bash
go get github.com/psyb0t/aichteeteapee
```

## Core Structures & Interfaces

### Server Package

```go
// Server is the main HTTP server
type Server struct {
    // Exported methods:
    Start(ctx context.Context, router *Router) error
    Stop(ctx context.Context) error
    GetRootGroup() *Group
    GetMux() *http.ServeMux
    GetHTTPListenerAddr() net.Addr
    GetHTTPSListenerAddr() net.Addr
    
    // Built-in handlers:
    HealthHandler(w http.ResponseWriter, r *http.Request)
    EchoHandler(w http.ResponseWriter, r *http.Request)
    FileUploadHandler(uploadsDir string, opts ...FileUploadHandlerOption) http.HandlerFunc
}

// Router defines your complete server configuration
type Router struct {
    GlobalMiddlewares []middleware.Middleware  // Applied to all routes
    Static           []StaticRouteConfig      // Static file serving configs  
    Groups           []GroupConfig            // Route groups
}

// StaticRouteConfig for serving static files
type StaticRouteConfig struct {
    Dir                   string                 // "./static" - directory to serve
    Path                  string                 // "/static" - URL path prefix
    DirectoryIndexingType DirectoryIndexingType  // HTML, JSON, or None
}

// GroupConfig for organizing routes
type GroupConfig struct {
    Path        string                    // "/api/v1" - group path prefix
    Middlewares []middleware.Middleware   // Group-specific middleware
    Routes      []RouteConfig             // Routes in this group
    Groups      []GroupConfig             // Nested groups (recursive)
}

// RouteConfig defines individual routes
type RouteConfig struct {
    Method  string           // http.MethodGet, http.MethodPost, etc.
    Path    string           // "/users/{id}" - route pattern
    Handler http.HandlerFunc // Your handler function
}

// Group provides fluent API for route registration
type Group struct {
    // Methods:
    Use(middlewares ...middleware.Middleware)
    Group(subPrefix string, middlewares ...middleware.Middleware) *Group
    Handle(method, pattern string, handler http.Handler, middlewares ...middleware.Middleware)
    HandleFunc(method, pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
    GET(pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
    POST(pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
    PUT(pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
    PATCH(pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
    DELETE(pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
    OPTIONS(pattern string, handler http.HandlerFunc, middlewares ...middleware.Middleware)
}

// Server constructors
func New() (*Server, error)
func NewWithLogger(logger *logrus.Logger) (*Server, error)
func NewWithConfig(config Config) (*Server, error)
func NewWithConfigAndLogger(config Config, logger *logrus.Logger) (*Server, error)
```

### WebSocket Package

```go
// Hub manages WebSocket clients and routes events
type Hub interface {
    Name() string
    Close()
    AddClient(client *Client)
    RemoveClient(clientID uuid.UUID)
    GetClient(clientID uuid.UUID) *Client
    GetOrCreateClient(clientID uuid.UUID, opts ...ClientOption) (*Client, bool)
    GetAllClients() map[uuid.UUID]*Client
    RegisterEventHandler(eventType EventType, handler EventHandler)
    RegisterEventHandlers(handlers map[EventType]EventHandler)
    UnregisterEventHandler(eventType EventType)
    ProcessEvent(client *Client, event *Event)
    BroadcastToAll(event *Event)
    BroadcastToClients(clientIDs []uuid.UUID, event *Event)
    BroadcastToSubscribers(eventType EventType, event *Event)
    Done() <-chan struct{}
}

// Client represents a WebSocket client with multiple connections
type Client struct {
    // Exported methods:
    ID() uuid.UUID
    SendEvent(event *Event)
    ConnectionCount() int
    GetConnections() map[uuid.UUID]*Connection
    IsSubscribedTo(eventType EventType) bool
}

// Connection represents a single WebSocket connection (clients can have multiple)
type Connection struct {
    // Exported methods:
    Send(event *Event)
    Stop()
    GetHubName() string
    GetClientID() uuid.UUID
}

// Event is the core message structure
type Event struct {
    ID        uuid.UUID         `json:"id"`        // Auto-generated UUID
    Type      EventType         `json:"type"`      // String event type 
    Data      json.RawMessage   `json:"data"`      // Your event payload
    Timestamp int64             `json:"timestamp"` // Unix timestamp
    Metadata  *EventMetadataMap `json:"metadata"`  // Key-value metadata
}

// EventHandler processes incoming events
type EventHandler func(hub Hub, client *Client, event *Event) error

// EventType is a string-based event type
type EventType string

// Built-in event types
const (
    EventTypeSystemLog   EventType = "system.log"
    EventTypeShellExec   EventType = "shell.exec"
    EventTypeEchoRequest EventType = "echo.request"
    EventTypeEchoReply   EventType = "echo.reply"
    EventTypeError       EventType = "error"
)

// WebSocket constructors and functions
func NewHub(name string) Hub
func NewEvent(eventType EventType, data any) *Event
func UpgradeHandler(hub Hub, opts ...HandlerOption) http.HandlerFunc
```

### Middleware Package

```go
// Middleware is just the standard http middleware pattern
type Middleware func(http.Handler) http.Handler

// Chain composes middlewares around a handler
func Chain(h http.Handler, middlewares ...Middleware) http.Handler

// Built-in middlewares (all return Middleware)
func Recovery() Middleware                    // Panic recovery
func RequestID() Middleware                   // Request ID generation  
func Logger(opts ...LoggerOption) Middleware // Request logging
func SecurityHeaders() Middleware             // Security headers (XSS, CSRF, etc.)
func CORS(opts ...CORSOption) Middleware     // CORS handling
func Timeout(duration time.Duration) Middleware // Request timeout
func BasicAuth(users map[string]string, opts ...BasicAuthOption) Middleware // Basic auth
func EnforceRequestContentType(contentType string) Middleware // Content-Type enforcement
```

## The Root Utilities (Use Anywhere)

The base `aichteeteapee` package gives you all the HTTP essentials:

```go
import "github.com/psyb0t/aichteeteapee"

// Pretty JSON responses with proper headers
aichteeteapee.WriteJSON(w, 200, map[string]string{"status": "winning"})

// Smart client IP extraction (handles proxies, load balancers, etc.)
clientIP := aichteeteapee.GetClientIP(r)

// Content type checking that actually works
if aichteeteapee.IsRequestContentTypeJSON(r) {
    // Handle JSON like a boss
}

// Request ID for tracing (if you set it in context)
requestID := aichteeteapee.GetRequestID(r)

// Predefined error responses that don't make you cry
aichteeteapee.WriteJSON(w, 404, aichteeteapee.ErrorResponseFileNotFound)
```

**What you get:**

- ✅ `WriteJSON()` - JSON responses with pretty formatting
- ✅ `GetClientIP()` - Smart IP extraction (X-Forwarded-For → X-Real-IP → RemoteAddr)
- ✅ `IsRequestContentTypeJSON/XML/FormData()` - Content type checking that works
- ✅ `GetRequestID()` - Request ID extraction from context
- ✅ HTTP header constants (`HeaderNameContentType`, etc.)
- ✅ Content type constants (`ContentTypeJSON`, etc.)
- ✅ Predefined error responses (`ErrorResponseBadRequest`, etc.)
- ✅ Context keys for request metadata

## The Full Server (Beast Mode)

Want everything? Here's a complete server setup:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "github.com/psyb0t/aichteeteapee/server"
    "github.com/psyb0t/aichteeteapee/server/middleware"
    "github.com/psyb0t/aichteeteapee/server/websocket"
)

func main() {
    // Create server
    s, err := server.New()
    if err != nil {
        log.Fatal(err)
    }

    // Create WebSocket hub for real-time features
    hub := websocket.NewHub("my-app")

    // Setup WebSocket event handlers
    hub.RegisterEventHandler(websocket.EventTypeEchoRequest, func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
        // Echo it back to the sender
        return client.SendEvent(websocket.NewEvent(websocket.EventTypeEchoReply, event.Data))
    })

    hub.RegisterEventHandler("file.delete", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
        type deleteMsg struct {
            FilePath string `json:"filePath"`
        }

        var msg deleteMsg
        json.Unmarshal(event.Data, &msg)

        // Do the file delete
        if err := os.Remove(msg.FilePath); err != nil {
            // Broadcast error to all clients
            hub.BroadcastToAll(websocket.NewEvent("file.delete.error", map[string]string{
                "error": err.Error(),
                "file":  msg.FilePath,
            }))
            return nil
        }

        // Broadcast success to all clients
        hub.BroadcastToAll(websocket.NewEvent("file.delete.success", map[string]string{
            "file": msg.FilePath,
        }))
        return nil
    })

    // Define your complete server structure
    router := &server.Router{
        GlobalMiddlewares: []middleware.Middleware{
            middleware.Recovery(),      // Panic recovery
            middleware.RequestID(),     // Request tracing
            middleware.Logger(),        // Request logging
            middleware.SecurityHeaders(), // Security headers
            middleware.CORS(),          // CORS handling
        },
        Static: []server.StaticRouteConfig{
            {
                Dir:  "./static",      // Serve static files
                Path: "/static",
            },
            {
                Dir:                   "./uploads",
                Path:                  "/files",
                DirectoryIndexingType: server.DirectoryIndexingTypeJSON, // Browseable uploads
            },
        },
        Groups: []server.GroupConfig{
            {
                Path: "/",
                Routes: []server.RouteConfig{
                    {
                        Method:  http.MethodGet,
                        Path:    "/",
                        Handler: func(w http.ResponseWriter, r *http.Request) {
                            w.Write([]byte("Welcome to the fucking show!"))
                        },
                    },
                    {
                        Method:  http.MethodGet,
                        Path:    "/ws",
                        Handler: websocket.UpgradeHandler(hub), // WebSocket endpoint
                    },
                    {
                        Method:  http.MethodPost,
                        Path:    "/upload",
                        Handler: s.FileUploadHandler("./uploads"), // File uploads
                    },
                },
            },
        },
    }

    // Start the beast
    log.Println("Starting server...")
    if err := s.Start(context.Background(), router); err != nil {
        log.Fatal(err)
    }
}
```

That's it. No, seriously. **THAT'S FUCKING IT.**

You now have:

- ✅ HTTP server on `:8080`
- ✅ HTTPS server on `:8443` (if you enable and configure TLS certs)
- ✅ CORS that doesn't hate you
- ✅ Request logging that makes sense
- ✅ Security headers that actually secure
- ✅ Static file serving from `./static`
- ✅ File uploads at `/upload` with UUID filenames
- ✅ Directory browsing at `/files` (JSON format)
- ✅ WebSocket support at `/ws`
- ✅ Panic recovery (your server won't die)
- ✅ Request ID tracing
- ✅ Graceful shutdown

## But I Want Simple Shit

Fine, you minimalist bastard:

```go
func main() {
    s, _ := server.New()

    router := &server.Router{
        Groups: []server.GroupConfig{
            {
                Path: "/",
                Routes: []server.RouteConfig{
                    {
                        Method: http.MethodGet,
                        Path:   "/",
                        Handler: func(w http.ResponseWriter, r *http.Request) {
                            aichteeteapee.WriteJSON(w, 200, map[string]string{
                                "message": "Hello World",
                            })
                        },
                    },
                },
            },
        },
    }

    s.Start(context.Background(), router)
}
```

## WebSocket Magic ✨

### Core Structures

```go
// Hub manages WebSocket clients and routes events
type Hub interface {
    Name() string
    Close()
    AddClient(client *Client)
    RemoveClient(clientID uuid.UUID)
    GetClient(clientID uuid.UUID) *Client
    GetOrCreateClient(clientID uuid.UUID, opts ...ClientOption) (*Client, bool)
    GetAllClients() map[uuid.UUID]*Client
    RegisterEventHandler(eventType EventType, handler EventHandler)
    RegisterEventHandlers(handlers map[EventType]EventHandler)
    UnregisterEventHandler(eventType EventType)
    ProcessEvent(client *Client, event *Event)
    BroadcastToAll(event *Event)
    BroadcastToClients(clientIDs []uuid.UUID, event *Event)
    BroadcastToSubscribers(eventType EventType, event *Event)
    Done() <-chan struct{}
}

// Client represents a WebSocket client with multiple connections
type Client struct {
    // Exported methods:
    ID() uuid.UUID
    SendEvent(event *Event)
    ConnectionCount() int
    GetConnections() map[uuid.UUID]*Connection
    IsSubscribedTo(eventType EventType) bool
}

// Event is the core message structure
type Event struct {
    ID        uuid.UUID         `json:"id"`        // Auto-generated UUID
    Type      EventType         `json:"type"`      // String event type 
    Data      json.RawMessage   `json:"data"`      // Your event payload
    Timestamp int64             `json:"timestamp"` // Unix timestamp
    Metadata  *EventMetadataMap `json:"metadata"`  // Key-value metadata
}

// EventHandler processes incoming events
type EventHandler func(hub Hub, client *Client, event *Event) error

// EventType is a string-based event type
type EventType string

// Built-in event types
const (
    EventTypeSystemLog   EventType = "system.log"
    EventTypeShellExec   EventType = "shell.exec"
    EventTypeEchoRequest EventType = "echo.request"
    EventTypeEchoReply   EventType = "echo.reply"
    EventTypeError       EventType = "error"
)
```

### Basic Hub Usage

```go
hub := websocket.NewHub("my-hub")

// Built-in echo handler for testing
hub.RegisterEventHandler(websocket.EventTypeEchoRequest, func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
    return client.SendEvent(websocket.NewEvent(websocket.EventTypeEchoReply, event.Data))
})

// Custom event handlers
hub.RegisterEventHandler("user.login", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
    // Process login event
    hub.BroadcastToAll(websocket.NewEvent("user.online", map[string]string{
        "userId": "123",
        "status": "online",
    }))
    return nil
})
```

### Event Structure

Events are the core data structure for WebSocket communication:

```go
type Event struct {
    ID        uuid.UUID         `json:"id"`        // Auto-generated UUID
    Type      EventType         `json:"type"`      // String event type 
    Data      json.RawMessage   `json:"data"`      // Your event payload
    Timestamp int64             `json:"timestamp"` // Unix timestamp
    Metadata  *EventMetadataMap `json:"metadata"`  // Key-value metadata
}

// Create events like this:
event := websocket.NewEvent("user.message", map[string]string{
    "message": "Hello world!",
    "username": "john",
})

// Or chain metadata:
event = websocket.NewEvent("system.alert", alertData).
    WithMetadata("priority", "high").
    WithMetadata("source", "monitoring")
```

### Multiple Hubs Pattern

You can run multiple specialized hubs for different purposes:

```go
func setupMultipleHubs() (websocket.Hub, websocket.Hub, websocket.Hub) {
    // Different hubs for different features
    chatHub := websocket.NewHub("chat-system")
    notificationHub := websocket.NewHub("notifications") 
    systemHub := websocket.NewHub("system-monitoring")
    
    // Chat hub handles user messages
    chatHub.RegisterEventHandler("chat.message", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
        type ChatMsg struct {
            Username string `json:"username"`
            Message  string `json:"message"`
            Room     string `json:"room"`
        }
        
        var msg ChatMsg
        json.Unmarshal(event.Data, &msg)
        
        // Broadcast to all clients in the chat hub
        return hub.BroadcastToAll(websocket.NewEvent("chat.broadcast", msg))
    })
    
    // Notification hub handles alerts
    notificationHub.RegisterEventHandler("alert.create", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
        // Only broadcast high-priority alerts
        if priority := event.Metadata.Get("priority"); priority == "high" {
            return hub.BroadcastToAll(event)
        }
        return client.SendEvent(event) // Send only to requesting client
    })
    
    // System hub handles monitoring
    systemHub.RegisterEventHandler("system.stats", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
        // Collect system stats and broadcast
        stats := map[string]any{
            "cpu": getCPUUsage(),
            "memory": getMemoryUsage(),
            "timestamp": time.Now().Unix(),
        }
        return hub.BroadcastToAll(websocket.NewEvent("system.update", stats))
    })
    
    return chatHub, notificationHub, systemHub
}

// Then in your router:
router := &server.Router{
    Groups: []server.GroupConfig{
        {
            Path: "/",
            Routes: []server.RouteConfig{
                {Method: http.MethodGet, Path: "/ws/chat", Handler: websocket.UpgradeHandler(chatHub)},
                {Method: http.MethodGet, Path: "/ws/notifications", Handler: websocket.UpgradeHandler(notificationHub)},
                {Method: http.MethodGet, Path: "/ws/system", Handler: websocket.UpgradeHandler(systemHub)},
            },
        },
    },
}
```

### Client Structure

In your event handlers, you get a `*websocket.Client` that represents the WebSocket client:

```go
hub.RegisterEventHandler("user.action", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
    // Client has these useful methods:
    
    clientID := client.ID()                    // Get client UUID
    connectionCount := client.ConnectionCount() // How many connections this client has
    
    // Send event only to this specific client
    client.SendEvent(websocket.NewEvent("response", responseData))
    
    // Check if client is subscribed to event types (always true by default)
    if client.IsSubscribedTo("notifications") {
        client.SendEvent(notificationEvent)
    }
    
    // Get all connections for this client (for advanced use cases)
    connections := client.GetConnections() // map[uuid.UUID]*Connection
    
    return nil
})
```

**Client vs Hub Broadcasting:**

```go
// Send to specific client only
client.SendEvent(event)

// Send to everyone in the hub
hub.BroadcastToAll(event)

// Send to specific clients by ID
hub.BroadcastToClients([]uuid.UUID{client1.ID(), client2.ID()}, event)

// Send to subscribers of a specific event type
hub.BroadcastToSubscribers("notifications", event)
```

**Multi-Connection Clients:**

A single client can have multiple WebSocket connections (e.g., multiple browser tabs):

```go
hub.RegisterEventHandler("user.status", func(hub websocket.Hub, client *websocket.Client, event *websocket.Event) error {
    connectionCount := client.ConnectionCount()
    
    if connectionCount > 1 {
        // User has multiple tabs open, send tab-specific response
        return client.SendEvent(websocket.NewEvent("multi.tab.warning", map[string]any{
            "message": fmt.Sprintf("You have %d tabs open", connectionCount),
        }))
    }
    
    return client.SendEvent(websocket.NewEvent("single.tab.response", responseData))
})
```

**Broadcasting options:**

- `hub.BroadcastToAll(event)` - Send to everyone
- `hub.BroadcastToClients([]uuid.UUID{id1, id2}, event)` - Send to specific clients
- `hub.BroadcastToSubscribers(eventType, event)` - Send to subscribers of event type

## Static Files & Uploads

**Static file serving:**

```go
Static: []server.StaticRouteConfig{
    {
        Dir:  "./public",
        Path: "/assets",
        DirectoryIndexingType: server.DirectoryIndexingTypeHTML, // or JSON, or None
    },
}
```

**File uploads with options:**

```go
Handler: s.FileUploadHandler("./uploads",
    server.WithFilenamePrependType(server.FilenamePrependTypeDateTime), // datetime_originalname.ext
    server.WithFileUploadHandlerPostprocessor(func(data map[string]any) (map[string]any, error) {
        // Process uploaded file data
        return data, nil
    }),
)
```

## Middleware System

**Built-in middleware:**

```go
GlobalMiddlewares: []middleware.Middleware{
    middleware.Recovery(),                    // Panic recovery
    middleware.RequestID(),                   // Request ID generation
    middleware.Logger(),                      // Request logging
    middleware.SecurityHeaders(),             // Security headers (XSS, CSRF, etc.)
    middleware.CORS(),                        // CORS with sensible defaults
    middleware.Timeout(30 * time.Second),     // Request timeout
    middleware.BasicAuth(map[string]string{"user": "pass"}), // Basic auth
}
```

**Per-group middleware:**

```go
Groups: []server.GroupConfig{
    {
        Path: "/admin",
        Middlewares: []middleware.Middleware{
            middleware.BasicAuth(adminUsers),
        },
        Routes: []server.RouteConfig{...},
    },
}
```

## Built-in Handlers

**Health check:**

```go
{
    Method:  http.MethodGet,
    Path:    "/health",
    Handler: s.HealthHandler, // Returns {"status": "ok"}
}
```

**Echo endpoint:**

```go
{
    Method:  http.MethodPost,
    Path:    "/echo",
    Handler: s.EchoHandler, // Echoes request body back
}
```

## Router Structure Deep Dive

The `Router` struct is how you declaratively configure your entire server:

```go
type Router struct {
    GlobalMiddlewares []middleware.Middleware  // Applied to all routes
    Static           []StaticRouteConfig      // Static file serving configs  
    Groups           []GroupConfig            // Route groups
}

type StaticRouteConfig struct {
    Dir                   string                 // "./static" - directory to serve
    Path                  string                 // "/static" - URL path prefix
    DirectoryIndexingType DirectoryIndexingType  // HTML, JSON, or None
}

type GroupConfig struct {
    Path        string                    // "/api/v1" - group path prefix
    Middlewares []middleware.Middleware   // Group-specific middleware
    Routes      []RouteConfig             // Routes in this group
    Groups      []GroupConfig             // Nested groups (recursive)
}

type RouteConfig struct {
    Method  string           // http.MethodGet, http.MethodPost, etc.
    Path    string           // "/users/{id}" - route pattern
    Handler http.HandlerFunc // Your handler function
}
```

**Complex router example:**

```go
router := &server.Router{
    // Global middleware applies to everything
    GlobalMiddlewares: []middleware.Middleware{
        middleware.Recovery(),
        middleware.RequestID(),
        middleware.Logger(),
        middleware.CORS(),
    },
    
    // Multiple static file routes
    Static: []server.StaticRouteConfig{
        {
            Dir:  "./public",
            Path: "/assets",
            DirectoryIndexingType: server.DirectoryIndexingTypeHTML,
        },
        {
            Dir:  "./uploads", 
            Path: "/files",
            DirectoryIndexingType: server.DirectoryIndexingTypeJSON,
        },
    },
    
    Groups: []server.GroupConfig{
        // Public routes (no auth)
        {
            Path: "/",
            Routes: []server.RouteConfig{
                {Method: http.MethodGet, Path: "/health", Handler: healthHandler},
                {Method: http.MethodGet, Path: "/ws", Handler: websocket.UpgradeHandler(hub)},
            },
        },
        
        // API with JSON enforcement
        {
            Path: "/api/v1",
            Middlewares: []middleware.Middleware{
                middleware.EnforceRequestContentType("application/json"),
            },
            Routes: []server.RouteConfig{
                {Method: http.MethodGet, Path: "/users", Handler: getUsersHandler},
                {Method: http.MethodPost, Path: "/users", Handler: createUserHandler},
            },
        },
        
        // Admin routes with auth
        {
            Path: "/admin",
            Middlewares: []middleware.Middleware{
                middleware.BasicAuth(map[string]string{"admin": "secret"}),
            },
            Routes: []server.RouteConfig{
                {Method: http.MethodGet, Path: "/stats", Handler: adminStatsHandler},
                {Method: http.MethodDelete, Path: "/users/{id}", Handler: deleteUserHandler},
            },
            
            // Nested group for super admin
            Groups: []server.GroupConfig{
                {
                    Path: "/super",
                    Middlewares: []middleware.Middleware{
                        superAdminAuthMiddleware,
                    },
                    Routes: []server.RouteConfig{
                        {Method: http.MethodPost, Path: "/reset", Handler: systemResetHandler},
                    },
                },
            },
        },
    },
}
```

## Configuration

Environment variables (with sensible defaults):

```bash
export HTTP_SERVER_LISTENADDRESS="0.0.0.0:8080"         # HTTP server address
export HTTP_SERVER_TLSENABLED="true"                    # Enable TLS/HTTPS
export HTTP_SERVER_TLSLISTENADDRESS="0.0.0.0:8443"     # HTTPS server address
export HTTP_SERVER_TLSCERTFILE="/path/to/cert.pem"     # TLS certificate file
export HTTP_SERVER_TLSKEYFILE="/path/to/key.pem"       # TLS private key file
export HTTP_SERVER_READTIMEOUT="30s"                   # Request read timeout
export HTTP_SERVER_WRITETIMEOUT="30s"                  # Response write timeout
export HTTP_SERVER_IDLETIMEOUT="60s"                   # Connection idle timeout
export HTTP_SERVER_FILEUPLOADMAXMEMORY="33554432"      # Max upload memory (bytes)
```

Or use custom config:

```go
s, err := server.NewWithConfig(server.Config{
    ListenAddress: "127.0.0.1:9000",
    ReadTimeout:   10 * time.Second,
    WriteTimeout:  10 * time.Second,
})
```

## Real Talk

This isn't another "minimal framework" that makes you implement everything. This is a **batteries-included HTTP utilities library** that handles the 90% of shit you always end up building anyway:

- 🔥 **Production-ready defaults** - TLS support, security headers, CORS, logging, graceful shutdown
- 🚀 **Zero-configuration startup** - Just create server, define routes, start
- 🛡️ **Security by default** - XSS protection, CSRF headers, secure defaults
- 📊 **Structured logging** - Request IDs, client IPs, timing, status codes
- 🌐 **CORS that works** - Sensible defaults, fully configurable
- 📁 **Static files + uploads** - Directory indexing, file caching, UUID filenames
- ⚡ **WebSocket support** - Hub system, event handling, broadcasting
- 🔧 **Completely customizable** - Override any default, add custom middleware
- 🧪 **90%+ test coverage** - Battle-tested and production-ready

## Why "aichteeteapee"?

Because saying "HTTP" is boring, but `aichteeteapee` makes you go "what the fuck is this?" and then you realize it's phonetically "HTTP" and you either laugh or hate it. Either way, you remember it.

Also, all the good names were taken.

## License

MIT - Because sharing is caring, and lawyers are expensive.

---

_"Finally, an HTTP library that doesn't make me want to switch careers."_ - Some Developer, Probably

_"I went from 200 lines of boilerplate to 20 lines of actual code."_ - Another Developer, Definitely

_"It just fucking works."_ - Everyone Who Uses This
