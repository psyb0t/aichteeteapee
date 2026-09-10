package wshub

import (
	"encoding/json"
	"testing"

	dabluveees "github.com/psyb0t/aichteeteapee/serbewr/dabluvee-es"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventTypeEchoRequestHandler(t *testing.T) {
	t.Parallel()

	client := NewClient()
	request := dabluveees.NewEvent(
		dabluveees.EventTypeEchoRequest,
		json.RawMessage(`{"message":"hello"}`),
	)

	err := EventTypeEchoRequestHandler(nil, client, request)
	require.NoError(t, err)

	reply := <-client.sendCh
	assert.Equal(t, dabluveees.EventTypeEchoReply, reply.Type)
	assert.JSONEq(t, string(request.Data), string(reply.Data))
	require.NotNil(t, reply.TriggeredBy)
	assert.Equal(t, request.ID, *reply.TriggeredBy)
}
