package wshub

import (
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	dabluveees "github.com/psyb0t/aichteeteapee/serbewr/dabluvee-es"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	invalidTestPing      dabluveees.EventType = "test.ping"
	invalidTestPong      dabluveees.EventType = "test.pong"
	invalidTestRejected  dabluveees.EventType = "test.rejected"
	invalidTestReadLimit                      = 64
	invalidTestWait                           = 3 * time.Second
)

var errInvalidTestRejected = errors.New("rejected by test handler")

type invalidEventCall struct {
	hub    Hub
	client *Client
	data   []byte
	err    error
}

type invalidEventRecorder struct {
	mu    sync.Mutex
	calls []invalidEventCall
}

func (r *invalidEventRecorder) record(call invalidEventCall) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, call)
}

func (r *invalidEventRecorder) snapshot() []invalidEventCall {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]invalidEventCall(nil), r.calls...)
}

//nolint:ireturn // NewHub exposes only the Hub interface.
func dialInvalidEventHub(
	t *testing.T, clientOptions ...ClientOption,
) (Hub, *websocket.Conn) {
	t.Helper()

	hub := NewHub("invalid-event-test")
	t.Cleanup(hub.Close)

	hub.RegisterEventHandler(invalidTestPing, func(
		_ Hub, client *Client, event *dabluveees.Event,
	) error {
		reply := dabluveees.NewEvent(invalidTestPong, nil).
			SetTriggeredBy(event.ID)
		client.SendEvent(&reply)

		return nil
	})

	server := httptest.NewServer(UpgradeHandler(hub,
		WithUpgradeHandlerClientOptions(clientOptions...)))
	t.Cleanup(server.Close)

	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)

	endpoint.Scheme = "ws"

	conn, response, err := websocket.DefaultDialer.DialContext(
		t.Context(), endpoint.String(), nil,
	)
	if response != nil {
		t.Cleanup(func() { assert.NoError(t, response.Body.Close()) })
	}

	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	require.Eventually(t, func() bool {
		return len(hub.GetAllClients()) == 1
	}, invalidTestWait, time.Millisecond)

	return hub, conn
}

func readInvalidTestEvent(
	t *testing.T, conn *websocket.Conn,
) dabluveees.Event {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(invalidTestWait)))

	var event dabluveees.Event
	require.NoError(t, conn.ReadJSON(&event))

	return event
}

func requireInvalidTestClose(
	t *testing.T, conn *websocket.Conn, code int,
) string {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(invalidTestWait)))

	// Events queued before the close frame may arrive first.
	var err error
	for err == nil {
		_, _, err = conn.ReadMessage()
	}

	var closeErr *websocket.CloseError
	require.ErrorAs(t, err, &closeErr)
	assert.Equal(t, code, closeErr.Code)

	return closeErr.Text
}

func pingInvalidTestHub(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	ping := dabluveees.NewEvent(invalidTestPing, nil)
	require.NoError(t, conn.WriteJSON(ping))

	reply := readInvalidTestEvent(t, conn)
	assert.Equal(t, invalidTestPong, reply.Type)
	require.NotNil(t, reply.TriggeredBy)
	assert.Equal(t, ping.ID, *reply.TriggeredBy)
}

func TestInvalidEventHandlerKeepsConnection(t *testing.T) {
	t.Parallel()

	recorder := &invalidEventRecorder{}
	handler := func(hub Hub, client *Client, data []byte, err error) error {
		recorder.record(invalidEventCall{
			hub: hub, client: client, data: data, err: err,
		})
		hub.BroadcastToClients(
			[]uuid.UUID{client.ID()},
			dabluveees.NewEvent(dabluveees.EventTypeError, nil),
		)

		return nil
	}

	hub, conn := dialInvalidEventHub(t, WithInvalidEventHandler(handler))

	pingInvalidTestHub(t, conn)

	frames := []string{
		`{"type": "test.ping"`,
		`{"id": "not-a-uuid", "type": "test.ping"}`,
		`{"type": 42}`,
	}
	for _, frame := range frames {
		require.NoError(t, conn.WriteMessage(
			websocket.TextMessage, []byte(frame),
		))

		reply := readInvalidTestEvent(t, conn)
		assert.Equal(t, dabluveees.EventTypeError, reply.Type)

		pingInvalidTestHub(t, conn)
	}

	calls := recorder.snapshot()
	require.Len(t, calls, len(frames))

	clients := hub.GetAllClients()
	require.Len(t, clients, 1)

	for index, call := range calls {
		assert.Equal(t, hub, call.hub)
		assert.Contains(t, clients, call.client.ID())
		assert.Equal(t, frames[index], string(call.data))
		assert.Error(t, call.err)
	}
}

func TestInvalidEventWithoutHandlerCloses(t *testing.T) {
	t.Parallel()

	hub, conn := dialInvalidEventHub(t)

	secret := "client-secret-payload"
	require.NoError(t, conn.WriteMessage(
		websocket.TextMessage, []byte("{"+secret),
	))

	closeText := requireInvalidTestClose(
		t, conn, websocket.CloseInvalidFramePayloadData,
	)
	assert.Equal(t, invalidEventCloseText, closeText)
	assert.NotContains(t, closeText, secret)

	assert.Eventually(t, func() bool {
		return len(hub.GetAllClients()) == 0
	}, invalidTestWait, time.Millisecond)
}

func TestInvalidEventHandlerErrorCloses(t *testing.T) {
	t.Parallel()

	_, conn := dialInvalidEventHub(t, WithInvalidEventHandler(
		func(hub Hub, client *Client, _ []byte, _ error) error {
			reply := dabluveees.NewEvent(invalidTestRejected, nil)
			hub.BroadcastToClients([]uuid.UUID{client.ID()}, reply)

			return errInvalidTestRejected
		},
	))

	require.NoError(t, conn.WriteMessage(
		websocket.TextMessage, []byte(`{"id": 7}`),
	))

	closeText := requireInvalidTestClose(
		t, conn, websocket.CloseInvalidFramePayloadData,
	)
	assert.Equal(t, invalidEventCloseText, closeText)
}

func TestOversizedFrameStillClosesTooBig(t *testing.T) {
	t.Parallel()

	called := make(chan struct{}, 1)
	_, conn := dialInvalidEventHub(t,
		WithReadLimit(invalidTestReadLimit),
		WithInvalidEventHandler(func(Hub, *Client, []byte, error) error {
			called <- struct{}{}

			return nil
		}),
	)

	payload := `{"type": "` + strings.Repeat("a", 2*invalidTestReadLimit) + `"}`
	require.NoError(t, conn.WriteMessage(
		websocket.TextMessage, []byte(payload),
	))

	_ = requireInvalidTestClose(t, conn, websocket.CloseMessageTooBig)
	assert.Empty(t, called)
}
