---
title: "WebSockets"
description: "The dabluvee-es packages: JSON events, the wshub hub, client and connection tiers, upgrade options, and the wsunixbridge Unix socket bridge."
nav_order: 4
---

# WebSockets

Pronounced "WS", because why stop at one wordplay.

[`serbewr/dabluvee-es`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr/dabluvee-es) (package `dabluveees`) defines the event type. Below it, [`wshub`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr/dabluvee-es/wshub) runs a three-tier hub, **Hub** -> **Client** -> **Connection**, and [`wsunixbridge`](https://github.com/psyb0t/aichteeteapee/tree/main/serbewr/dabluvee-es/wsunixbridge) pipes a WebSocket to Unix sockets.

## Quick start

```go
import (
    dabluveees "github.com/psyb0t/aichteeteapee/serbewr/dabluvee-es"
    "github.com/psyb0t/aichteeteapee/serbewr/dabluvee-es/wshub"
)

hub := wshub.NewHub("chat")
defer hub.Close()

hub.RegisterEventHandler("chat.message", func(
    h wshub.Hub, client *wshub.Client, event *dabluveees.Event,
) error {
    h.BroadcastToAll(event)

    return nil
})

srv.GetRootGroup().GET("/ws/chat", wshub.UpgradeHandler(hub))
```

A browser connects to `/ws/chat`, sends `{"type": "chat.message", "data": {"text": "hello"}}`, and every connected client gets it back.

## Events

Everything on the wire is a JSON event:

```json
{
  "id": "0b9f3c1e-7a52-4c1d-9d0e-3e6f2a8b4c71",
  "type": "chat.message",
  "data": {
    "text": "hello"
  },
  "timestamp": 1767225600,
  "metadata": {
    "room": "general"
  },
  "triggeredBy": null
}
```

| Field | Go type | Meaning |
| --- | --- | --- |
| `id` | `uuid.UUID` | UUID4, for tracing. |
| `type` | `EventType` | A string you pick. Handlers are registered per type. |
| `data` | `json.RawMessage` | The payload, marshalled once and passed along as raw bytes. |
| `timestamp` | `int64` | Unix seconds, set by whoever sends the event. |
| `metadata` | `*EventMetadataMap` | Key-value map safe for concurrent use (`Set`, `Get`, `GetAll`, `Copy`). |
| `triggeredBy` | `*uuid.UUID` | ID of the event that caused this one, if any. |

Build one with `NewEvent`, which fills in the ID, the timestamp and an empty metadata map and returns a `*Event`:

```go
event := dabluveees.NewEvent("chat.message", map[string]string{
    "text": "hello",
})
hub.BroadcastToAll(event)
```

`SetMetadata`, `SetTimestamp` and `SetTriggeredBy` chain. They return an `Event` value rather than a pointer, so take its address to send it. The copy shares the original's metadata map, so `SetMetadata` changes both.

```go
reply := dabluveees.NewEvent("chat.ack", nil).
    SetMetadata("room", "general").
    SetTriggeredBy(event.ID)
client.SendEvent(&reply)
```

`GetTime()` converts the timestamp to a `time.Time`, and `IsRecent(seconds)` says whether it is within the last `seconds` seconds.

The package names a few event types: `EventTypeSystemLog` (`system.log`), `EventTypeShellExec` (`shell.exec`), `EventTypeEchoRequest` (`echo.request`), `EventTypeEchoReply` (`echo.reply`) and `EventTypeError` (`error`). They are just names. Only the echo pair has a ready-made handler, `wshub.EventTypeEchoRequestHandler`, which answers an `echo.request` with an `echo.reply` carrying the same data. Register it if you want it:

```go
hub.RegisterEventHandler(
    dabluveees.EventTypeEchoRequest,
    wshub.EventTypeEchoRequestHandler,
)
```

## Hub

`wshub.NewHub(name)` returns a `Hub`. It owns the clients and routes each incoming event to the handler registered for its type.

```go
hub.RegisterEventHandler("chat.message", messageHandler)
hub.RegisterEventHandlers(map[dabluveees.EventType]wshub.EventHandler{
    "chat.message": messageHandler,
    "chat.join":    joinHandler,
})
hub.UnregisterEventHandler("chat.join")

hub.BroadcastToAll(event)
hub.BroadcastToClients([]uuid.UUID{id1, id2}, event)
hub.BroadcastToSubscribers("chat.message", event)

client := hub.GetClient(id)       // nil if unknown
clients := hub.GetAllClients()    // a copy, safe to range over

hub.Close() // stops every client and waits for their goroutines
<-hub.Done()
```

An `EventHandler` is `func(hub Hub, client *Client, event *dabluveees.Event) error`. An event with no handler is ignored, and a handler error is logged, not sent to the client. Reply with `client.SendEvent` or a broadcast.

`BroadcastToSubscribers` currently reaches every client: there is no subscription tracking yet, and `Client.IsSubscribedTo` always says yes.

## Client

A client is one logical user. It can hold several WebSocket connections (a few browser tabs, a phone and a laptop) and every event sent to the client goes out on all of them.

The upgrade handler picks the client ID from the `clientID` query parameter, then the `X-Client-ID` header. A connection with a known ID joins that client; an unknown ID creates the client; no ID, or one that is not a valid UUID, gets a fresh client with a new UUID.

Client options are passed through the upgrade handler (or to `wshub.NewClient` and `hub.GetOrCreateClient` when you build clients yourself):

```go
wshub.UpgradeHandler(hub,
    wshub.WithUpgradeHandlerClientOptions(
        wshub.WithSendBufferSize(512),
        wshub.WithReadTimeout(60*time.Second),
        wshub.WithWriteTimeout(10*time.Second),
        wshub.WithPingInterval(54*time.Second),
        wshub.WithPongTimeout(60*time.Second),
        wshub.WithReadLimit(1024*1024),
    ),
)
```

| Option | Default | Effect |
| --- | --- | --- |
| `WithSendBufferSize` | 256 | Queued events per client and per connection. When a client's queue is full, new events for it are dropped with a warning. |
| `WithReadLimit` | 1MB | Largest incoming message. A bigger one closes the connection. |
| `WithReadTimeout` | 60s | Read deadline, reset after every message. |
| `WithPongTimeout` | 60s | Read deadline set after each pong. |
| `WithPingInterval` | 54s | How often the server pings. |
| `WithWriteTimeout` | 10s | Write deadline for each message and ping. |
| `WithReadBufferSize`, `WithWriteBufferSize` | 1024 | Stored on the client but not used. Set socket buffer sizes on the upgrade handler instead. |

## Connection

Each connection wraps a `gorilla/websocket` conn with two goroutines. The write pump sends queued events and pings. The read pump decodes each incoming message as an event and hands it to the hub. A message that is not a valid event closes the connection, and so does a missed pong. When a connection's pumps stop, the connection removes itself from its client.

## Upgrade handler

```go
wshub.UpgradeHandler(hub,
    wshub.WithUpgradeHandlerBufferSizes(2048, 4096),
    wshub.WithUpgradeHandlerHandshakeTimeout(30*time.Second),
    wshub.WithUpgradeHandlerCompression(true),
    wshub.WithUpgradeHandlerSubprotocols("chat", "json"),
    wshub.WithUpgradeHandlerCheckOrigin(customCheckFn),
    wshub.WithUpgradeHandlerClientOptions(
        wshub.WithSendBufferSize(512),
    ),
)
```

| Option | Default |
| --- | --- |
| `WithUpgradeHandlerBufferSizes(read, write)` | 1024, 1024 bytes |
| `WithUpgradeHandlerHandshakeTimeout` | 45s |
| `WithUpgradeHandlerCompression` | off |
| `WithUpgradeHandlerSubprotocols` | none |
| `WithUpgradeHandlerCheckOrigin` | `aichteeteapee.GetDefaultWebSocketCheckOrigin` |

The default origin check accepts a request with no `Origin` header and otherwise requires the `Origin` host to equal the request `Host`, because that's what secure means. Use `aichteeteapee.GetPermissiveWebSocketCheckOrigin` for one handler, or `aichteeteapee.FuckSecurity()` for the whole process, when you're just trying to get stuff working locally. See [Security](security.md).

A failed upgrade gets gorilla's own HTTP error response.

## Unix socket bridge

`wsunixbridge` bridges a WebSocket connection to a pair of Unix domain sockets, for when you need to plug in external tools: shells, processes, whatever speaks bytes over a socket.

```go
handler := wsunixbridge.NewUpgradeHandler(
    "/var/run/app/sockets",
    func(conn *wsunixbridge.Connection) error {
        // conn.WriterUnixSock.Path: external tools read WebSocket data here
        // conn.ReaderUnixSock.Path: external tools write here to send to the WebSocket
        return nil
    },
)

srv.GetRootGroup().GET("/ws/bridge", handler)
```

For each connection the bridge:

1. creates the sockets directory with mode `0700` if needed, and two sockets in it, `<connection-id>_output` and `<connection-id>_input`, each with mode `0600`;
2. sends the WebSocket client a `wsunixbridge.init` event whose data holds `writerSocket` and `readerSocket`, the two paths;
3. calls your handler in its own goroutine (an error is logged);
4. copies every text or binary WebSocket message to all processes connected to the `_output` socket, and sends whatever a process writes to the `_input` socket to the WebSocket as binary messages.

When the WebSocket closes, the bridge closes both sockets and removes the files. It uses the default upgrade settings, including the default origin check, and takes no options.
