package wshub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent(t *testing.T) {
	testData := map[string]any{
		"message": "hello world",
		"userID":  123,
	}

	event := NewEvent(EventTypeSystemLog, testData)

	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.Equal(t, EventTypeSystemLog, event.Type)
	assert.NotNil(t, event.Data)
	assert.NotEqual(t, int64(0), event.Timestamp)
	assert.NotNil(t, event.Metadata)
	assert.Len(t, event.Metadata.GetAll(), 0)

	// Test timestamp is recent (within last 5 seconds)
	now := time.Now().Unix()
	assert.True(t, event.Timestamp >= now-5 && event.Timestamp <= now+1)

	// Test data marshaling worked
	var unmarshaled map[string]any

	err := json.Unmarshal(event.Data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "hello world", unmarshaled["message"])
	// JSON unmarshals numbers as float64
	assert.Equal(t, float64(123), unmarshaled["userID"])
}

func TestNewEvent_NilData(t *testing.T) {
	event := NewEvent(EventTypeError, nil)

	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.Equal(t, EventTypeError, event.Type)
	assert.Nil(t, event.Data)
	assert.NotEqual(t, int64(0), event.Timestamp)
	assert.NotNil(t, event.Metadata)
}

func TestNewEvent_InvalidData(t *testing.T) {
	// Create data that can't be marshaled (function)
	invalidData := func() {}

	event := NewEvent(EventTypeError, invalidData)

	assert.NotEqual(t, uuid.Nil, event.ID)
	assert.Equal(t, EventTypeError, event.Type)
	assert.Nil(t, event.Data) // Should be nil because marshaling failed
	assert.NotEqual(t, int64(0), event.Timestamp)
	assert.NotNil(t, event.Metadata)
}

func TestEvent_WithMetadata(t *testing.T) {
	event := NewEvent(EventTypeSystemLog, "test")

	// Test chaining - this modifies the event in place
	result := event.WithMetadata("userID", "user123").
		WithMetadata("room", "general")

	userID, exists := result.Metadata.Get("userID")
	assert.True(t, exists)
	assert.Equal(t, "user123", userID)

	room, exists := result.Metadata.Get("room")
	assert.True(t, exists)
	assert.Equal(t, "general", room)

	assert.Len(t, result.Metadata.GetAll(), 2)

	// Since we're modifying in place, the original event now has the metadata too
	assert.Len(t, event.Metadata.GetAll(), 2)
}

func TestEvent_WithMetadata_NilMetadata(t *testing.T) {
	event := Event{
		ID:        uuid.New(),
		Type:      EventTypeSystemLog,
		Metadata:  nil, // Explicitly nil
		Timestamp: time.Now().Unix(),
	}

	result := event.WithMetadata("test", "value")

	value, exists := result.Metadata.Get("test")
	assert.True(t, exists)
	assert.Equal(t, "value", value)
	assert.Len(t, result.Metadata.GetAll(), 1)
}

func TestEvent_WithTimestamp(t *testing.T) {
	event := NewEvent(EventTypeSystemLog, "test")
	originalTimestamp := event.Timestamp

	customTimestamp := int64(1640995200) // 2022-01-01 00:00:00 UTC
	result := event.WithTimestamp(customTimestamp)

	assert.Equal(t, customTimestamp, result.Timestamp)
	assert.Equal(t, originalTimestamp, event.Timestamp) // Original unchanged
}

func TestEvent_GetTime(t *testing.T) {
	timestamp := int64(1640995200) // 2022-01-01 00:00:00 UTC
	event := NewEvent(EventTypeSystemLog, "test").WithTimestamp(timestamp)

	result := event.GetTime()

	expected := time.Unix(timestamp, 0)
	assert.Equal(t, expected, result)
	assert.Equal(t, 2022, result.Year())
	assert.Equal(t, time.January, result.Month())
	assert.Equal(t, 1, result.Day())
}

func TestEvent_IsRecent(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name      string
		timestamp int64
		seconds   int64
		expected  bool
	}{
		{
			name:      "current time is recent",
			timestamp: now,
			seconds:   60,
			expected:  true,
		},
		{
			name:      "30 seconds ago is recent within 60 seconds",
			timestamp: now - 30,
			seconds:   60,
			expected:  true,
		},
		{
			name:      "exactly at boundary is recent",
			timestamp: now - 60,
			seconds:   60,
			expected:  true,
		},
		{
			name:      "61 seconds ago is not recent within 60 seconds",
			timestamp: now - 61,
			seconds:   60,
			expected:  false,
		},
		{
			name:      "future timestamp is recent",
			timestamp: now + 10,
			seconds:   60,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(EventTypeSystemLog, "test").WithTimestamp(tt.timestamp)
			result := event.IsRecent(tt.seconds)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvent_JSONMarshaling(t *testing.T) {
	testData := map[string]any{
		"message": "hello",
		"count":   42,
	}

	event := NewEvent(EventTypeSystemLog, testData).
		WithMetadata("userID", "user123").
		WithTimestamp(1640995200)

	// Marshal to JSON
	jsonData, err := json.Marshal(event)
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled Event

	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, event.ID, unmarshaled.ID)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.Equal(t, event.Timestamp, unmarshaled.Timestamp)
	assert.Equal(t, event.Metadata.GetAll(), unmarshaled.Metadata.GetAll())
	assert.Equal(t, event.Data, unmarshaled.Data)

	// Verify data content
	var originalData, unmarshaledData map[string]any

	err = json.Unmarshal(event.Data, &originalData)
	require.NoError(t, err)
	err = json.Unmarshal(unmarshaled.Data, &unmarshaledData)
	require.NoError(t, err)
	assert.Equal(t, originalData, unmarshaledData)
}

func TestNewEvent_UniqueIDs(t *testing.T) {
	// Create multiple events and verify they all have unique IDs
	events := make([]*Event, 100)
	ids := make(map[uuid.UUID]bool)

	for i := range 100 {
		events[i] = NewEvent(EventTypeSystemLog, "test")

		// Verify ID is not nil
		assert.NotEqual(t, uuid.Nil, events[i].ID)

		// Verify ID is unique
		assert.False(t, ids[events[i].ID], "Duplicate ID found: %s", events[i].ID)
		ids[events[i].ID] = true
	}

	// Verify we have 100 unique IDs
	assert.Len(t, ids, 100)
}
