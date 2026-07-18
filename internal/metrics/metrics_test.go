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
	assert.Same(t, m1, m2) // тот же экземпляр, иначе promauto паникует на второй регистрации
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

// методы записи не должны паниковать на нормальных аргументах
func TestRecordMethods_NoPanic(t *testing.T) {
	m := New()

	assert.NotPanics(t, func() {
		m.RecordMatchStart()
		m.RecordMatchComplete("dilemma", "completed", 5*time.Second)
		m.RecordHTTPRequest("GET", "/api/v1/tournaments", "200", 50*time.Millisecond)
		m.RecordDBQuery("select", 2*time.Millisecond)
		m.RecordCacheHit("get")
		m.RecordCacheMiss("get")
		m.SetQueueSize("high", 42)
		m.SetActiveWorkers(5)
		m.SetWorkerPoolSize(10)
		m.SetDBConnections(5, 10, 15)
	})
}
