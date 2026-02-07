package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Singleton(t *testing.T) {
	m1 := New()
	m2 := New()

	require.NotNil(t, m1)
	assert.Same(t, m1, m2) // Same instance
}

func TestNew_AllFieldsNotNil(t *testing.T) {
	m := New()

	assert.NotNil(t, m.MatchesTotal)
	assert.NotNil(t, m.MatchDuration)
	assert.NotNil(t, m.MatchesInProgress)
	assert.NotNil(t, m.QueueSize)
	assert.NotNil(t, m.QueueWaitTime)
	assert.NotNil(t, m.ActiveWorkers)
	assert.NotNil(t, m.WorkerPoolSize)
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDuration)
	assert.NotNil(t, m.HTTPRequestsInFlight)
	assert.NotNil(t, m.DBQueryDuration)
	assert.NotNil(t, m.DBConnections)
	assert.NotNil(t, m.CacheHits)
	assert.NotNil(t, m.CacheMisses)
}

func TestRecordMatchStart_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.RecordMatchStart()
	})
}

func TestRecordMatchComplete_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.RecordMatchComplete("dilemma", "completed", 5*time.Second)
	})
}

func TestRecordHTTPRequest_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.RecordHTTPRequest("GET", "/api/v1/tournaments", "200", 50*time.Millisecond)
	})
}

func TestRecordDBQuery_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.RecordDBQuery("select", 2*time.Millisecond)
	})
}

func TestRecordCacheHit_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.RecordCacheHit("get")
	})
}

func TestRecordCacheMiss_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.RecordCacheMiss("get")
	})
}

func TestSetQueueSize_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.SetQueueSize("high", 42)
	})
}

func TestSetActiveWorkers_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.SetActiveWorkers(5)
	})
}

func TestSetWorkerPoolSize_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.SetWorkerPoolSize(10)
	})
}

func TestSetDBConnections_NoPanic(t *testing.T) {
	m := New()
	assert.NotPanics(t, func() {
		m.SetDBConnections(5, 10, 15)
	})
}
