package wshub

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventMetadataMap(t *testing.T) {
	emm := newEventMetadataMap()

	// Test Set and Get
	emm.Set("key1", "value1")
	emm.Set("key2", 42)

	value, exists := emm.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)

	value, exists = emm.Get("key2")
	assert.True(t, exists)
	assert.Equal(t, 42, value)

	// Test non-existent key
	value, exists = emm.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, value)

	// Test GetAll
	all := emm.GetAll()
	expected := map[string]any{
		"key1": "value1",
		"key2": 42,
	}
	assert.Equal(t, expected, all)
}

func TestEventMetadataMap_Copy(t *testing.T) {
	original := newEventMetadataMap()
	original.Set("key1", "value1")
	original.Set("key2", 42)

	copied := original.Copy()

	// Verify copy has same data
	assert.Equal(t, original.GetAll(), copied.GetAll())

	// Verify they're independent
	copied.Set("key3", "new value")

	originalData := original.GetAll()
	copyData := copied.GetAll()

	assert.Len(t, originalData, 2)
	assert.Len(t, copyData, 3)
	assert.Equal(t, "new value", copyData["key3"])

	_, exists := originalData["key3"]
	assert.False(t, exists)
}

func TestEventMetadataMap_JSON(t *testing.T) {
	emm := newEventMetadataMap()
	emm.Set("string", "hello")
	emm.Set("number", 42)
	emm.Set("bool", true)

	// Marshal to JSON
	jsonData, err := json.Marshal(emm)
	require.NoError(t, err)

	// Unmarshal back
	var unmarshaled EventMetadataMap

	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Verify data (JSON unmarshaling converts numbers to float64)
	unmarshaledData := unmarshaled.GetAll()
	assert.Equal(t, "hello", unmarshaledData["string"])
	assert.Equal(t, float64(42), unmarshaledData["number"])
	assert.Equal(t, true, unmarshaledData["bool"])
	assert.Len(t, unmarshaledData, 3)
}

func TestEventMetadataMap_Concurrency(_ *testing.T) {
	emm := newEventMetadataMap()

	var wg sync.WaitGroup

	// Concurrent writes
	for i := range 100 {
		wg.Add(1)

		go func(val int) {
			defer wg.Done()

			emm.Set("key", val)
		}(i)
	}

	// Concurrent reads
	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			emm.Get("key")
			emm.GetAll()
		}()
	}

	wg.Wait()
	// If we get here without race conditions, test passes
}

func TestEventMetadataMap_UnmarshalError(t *testing.T) {
	emm := newEventMetadataMap()

	// Try to unmarshal invalid JSON
	invalidJSON := []byte("{invalid json")
	err := emm.UnmarshalJSON(invalidJSON)

	// Should return an error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal metadata")
}
