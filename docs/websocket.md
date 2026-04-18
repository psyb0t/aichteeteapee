# serbewr/dabluvee-es — WebSocket event system

Pronounced "WS" — because why stop at one wordplay.

Three-tier architecture: **Hub** -> **Client** -> **Connection**.

## Quick start

```go
hub := wshub.NewHub("chat")

hub.RegisterEventHandler("chat.message", func(
    h wshub.Hub, client *wshub.Client, event *dabluveees.Event,
) error {
    h.BroadcastToAll(event)
    return nil
})

rootGroup.GET("/ws/chat", wshub.UpgradeHandler(hub))
```

## Events

```go
event := dabluveees.NewEvent("chat.message", map[string]string{
    "text": "hello",
})
```

Each event has:
- `ID` — UUID4 for tracing
- `Type` — typed string (`EventType`)
- `Data` — `json.RawMessage` (zero-copy, no double-marshal)
- `Timestamp` — Unix seconds, set by sender
- `Metadata` — thread-safe key-value map
- `TriggeredBy` — optional reference to a parent event

Built-in types: `EventTypeSystemLog`, `EventTypeShellExec`, `EventTypeEchoRequest`/`Reply`, `EventTypeError`.

Chainable setters:

```go
event := dabluveees.NewEvent("chat.message", data).
    SetMetadata("room", "general").
    SetTriggeredBy(parentEventID)
```

## Hub

Manages clients, routes events to registered handlers, broadcasts.

```go
hub := wshub.NewHub("chat")

hub.RegisterEventHandler("chat.message", handler)
hub.RegisterEventHandlers(map[dabluveees.EventType]wshub.EventHandler{
    "chat.message": messageHandler,
    "chat.join":    joinHandler,
})

hub.BroadcastToAll(event)
hub.BroadcastToClients([]uuid.UUID{id1, id2}, event)
hub.BroadcastToSubscribers("chat.message", event)

hub.Close() // stops all clients, waits for goroutines
```

## Client

Represents a logical user. One client can have multiple WebSocket connections — multiple browser tabs, a phone and a laptop, whatever. Events sent to a client get distributed to all of them.

```go
client := wshub.NewClient(
    wshub.WithSendBufferSize(512),
    wshub.WithReadTimeout(60 * time.Second),
    wshub.WithWriteTimeout(10 * time.Second),
    wshub.WithPingInterval(54 * time.Second),
    wshub.WithPongTimeout(60 * time.Second),
    wshub.WithReadLimit(1024 * 1024),
)
```

Client ID can be provided via `?clientID=<uuid>` query param or `X-Client-ID` header. If not provided, a new UUID is generated. Multiple connections with the same client ID are grouped under one client.

## Connection

Wraps `gorilla/websocket` with:
- Write pump with ping/pong heartbeat
- Read pump with configurable read limits and timeouts
- Atomic shutdown with `sync.Once`
- In-flight send tracking via `WaitGroup`

## Upgrade handler

```go
wshub.UpgradeHandler(hub,
    wshub.WithUpgradeHandlerBufferSizes(2048, 4096),
    wshub.WithUpgradeHandlerHandshakeTimeout(30 * time.Second),
    wshub.WithUpgradeHandlerCompression(true),
    wshub.WithUpgradeHandlerSubprotocols("chat", "json"),
    wshub.WithUpgradeHandlerCheckOrigin(customCheckFn),
    wshub.WithUpgradeHandlerClientOptions(
        wshub.WithSendBufferSize(512),
    ),
)
```

Default `CheckOrigin` validates `Origin` against `Host`, because that's what secure means. Use `aichteeteapee.GetPermissiveWebSocketCheckOrigin` or `aichteeteapee.FuckSecurity()` when you're just trying to get stuff working locally.

## Unix Socket Bridge

`wsunixbridge` bridges WebSocket connections to Unix domain sockets for when you need to plug in external tools — shells, processes, whatever speaks bytes over a socket.

```go
handler := wsunixbridge.NewUpgradeHandler(
    "/var/run/app/sockets",
    func(conn *wsunixbridge.Connection) error {
        // conn.WriterUnixSock — external tools read WS data from here
        // conn.ReaderUnixSock — external tools write data here to send to WS
        return nil
    },
)
```

Each connection gets a dedicated pair of Unix sockets. Socket files are created with `0600` permissions in a `0700` directory.
