package scenarios

// =============================================================================
// ESSENTIAL PROCESS: Integration scenarios validating flexible-logger resilience,
// rate-throttled Telegram buffer-drop alerts, SSoT Cap'n Proto wire fidelity, and
// bounded shutdown drain timeouts under load.
//
// DATA FLOW:
//   1. Emits bursts of log entries through AsyncSink to saturate capacity.
//   2. Asserts atomic CAS throttling delivers <= 1 alert per window tagged 'telegram'.
//   3. Encodes log entries using SSoT Cap'n Proto schemas and verifies zero-loss decoding.
//   4. Simulates hanging downstream writers and asserts Close() terminates within timeout.
//
// KEY PARAMETERS:
//   - TestLoggerResilience_ThrottledTelegramDropAlert: Buffer saturation & Telegram gating.
//   - TestLoggerResilience_CanonicalSchemaRoundtrip: SSoT wire schema validation.
//   - TestLoggerResilience_BoundedDrainTimeoutShutdown: Graceful shutdown timeout verification.
// =============================================================================

import (
	"strings"
	"sync"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Bastien-Antigravity/flexible-logger/src/models"
	"github.com/Bastien-Antigravity/flexible-logger/src/serializers"
	"github.com/Bastien-Antigravity/flexible-logger/src/sink"
	logger_schema "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/schemas/logger"
)

// -----------------------------------------------------------------------------

type mockScenarioNotifier struct {
	mu     sync.Mutex
	alerts []*models.NotifMessage
}

func (m *mockScenarioNotifier) Notify(alert *models.NotifMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, alert)
	return nil
}

func (m *mockScenarioNotifier) Close() error { return nil }

func (m *mockScenarioNotifier) GetAlerts() []*models.NotifMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*models.NotifMessage, len(m.alerts))
	copy(out, m.alerts)
	return out
}

// -----------------------------------------------------------------------------

type discardScenarioSink struct{}

func (d *discardScenarioSink) Write(e *models.LogEntry) error {
	e.Release()
	return nil
}

func (d *discardScenarioSink) Close() error { return nil }

// -----------------------------------------------------------------------------

type slowScenarioSink struct {
	block chan struct{}
}

func (s *slowScenarioSink) Write(e *models.LogEntry) error {
	<-s.block
	e.Release()
	return nil
}

func (s *slowScenarioSink) Close() error { return nil }

// -----------------------------------------------------------------------------

// TestLoggerResilience_ThrottledTelegramDropAlert verifies that saturating the
// AsyncSink queue emits at most 1 warning alert per throttle window tagged 'telegram'.
func TestLoggerResilience_ThrottledTelegramDropAlert(t *testing.T) {
	mockNotif := &mockScenarioNotifier{}
	downstream := &discardScenarioSink{}

	// Create an AsyncSink with minimal capacity 1
	asyncSink := sink.NewAsyncSink(downstream, 1)
	asyncSink.SetDropAlertInterval(10 * time.Second)
	asyncSink.SetDropNotifier(mockNotif, "order-gateway")
	defer asyncSink.Close()

	// Saturate queue with 50 log entries to trigger drop policy
	for i := 0; i < 50; i++ {
		entry := &models.LogEntry{
			Level:   models.LevelInfo,
			Message: "high-frequency log entry",
		}
		_ = asyncSink.Write(entry)
	}

	// Allow worker goroutines to process
	time.Sleep(50 * time.Millisecond)

	alerts := mockNotif.GetAlerts()
	require.NotEmpty(t, alerts, "At least one drop alert must be emitted upon saturation")
	assert.Equal(t, 1, len(alerts), "Throttling must restrict alerts to exactly 1 within the window")

	alert := alerts[0]
	assert.Equal(t, "WARNING", alert.Level)
	assert.Contains(t, alert.Tags, "telegram", "Drop alert must route to Telegram")
	assert.Contains(t, alert.Message, "AsyncSink buffer saturated", "Alert must report saturation")
	assert.Greater(t, asyncSink.DroppedCount(), uint64(0), "Cumulative drops must be tracked")
}

// -----------------------------------------------------------------------------

// TestLoggerResilience_CanonicalSchemaRoundtrip verifies that Cap'n Proto wire
// serialization adheres strictly to the canonical SSoT schema in microservice-toolbox.
func TestLoggerResilience_CanonicalSchemaRoundtrip(t *testing.T) {
	serializer := serializers.NewCapnpSerializer()

	entry := &models.LogEntry{
		Timestamp:    time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC),
		Level:        models.LevelWarning,
		Message:      "critical trade execution delay",
		Hostname:     "gateway-node-01",
		LoggerName:   "orderbook-aggregator",
		ServiceName:  "orderbook-aggregator",
		ProcessID:    "94102",
		ProcessName:  "orderbook-aggregator",
		Filename:     "matcher.go",
		FunctionName: "ExecuteMatch",
		LineNumber:   "142",
	}

	data, err := serializer.Serialize(entry)
	require.NoError(t, err, "Serialization must succeed")
	require.NotEmpty(t, data, "Serialized binary must not be empty")

	// Decode using canonical SSoT reader from microservice-toolbox
	msg, err := capnp.UnmarshalPacked(data)
	require.NoError(t, err, "Unmarshaling packed Cap'n Proto must succeed")

	logMsg, err := logger_schema.ReadRootLoggerMsg(msg)
	require.NoError(t, err, "Reading root LoggerMsg must succeed")

	msgText, err := logMsg.Message_()
	require.NoError(t, err)
	assert.Equal(t, "critical trade execution delay", msgText)

	hostText, err := logMsg.Hostname()
	require.NoError(t, err)
	assert.Equal(t, "gateway-node-01", hostText)

	loggerNameText, err := logMsg.LoggerName()
	require.NoError(t, err)
	assert.Equal(t, "orderbook-aggregator", loggerNameText)

	svcText, err := logMsg.ServiceName()
	require.NoError(t, err)
	assert.Equal(t, "orderbook-aggregator", svcText)

	fileText, err := logMsg.Filename()
	require.NoError(t, err)
	assert.Equal(t, "matcher.go", fileText)

	funcText, err := logMsg.FunctionName()
	require.NoError(t, err)
	assert.Equal(t, "ExecuteMatch", funcText)

	lineText, err := logMsg.LineNumber()
	require.NoError(t, err)
	assert.Equal(t, "142", lineText)

	pidText, err := logMsg.ProcessId()
	require.NoError(t, err)
	assert.Equal(t, "94102", pidText)

	assert.Equal(t, uint16(models.LevelWarning), uint16(logMsg.Level()))
}

// -----------------------------------------------------------------------------

// TestLoggerResilience_BoundedDrainTimeoutShutdown verifies that AsyncSink.Close()
// enforces bounded drain timeout and unblocks without deadlocking when downstream sinks hang.
func TestLoggerResilience_BoundedDrainTimeoutShutdown(t *testing.T) {
	hangSink := &slowScenarioSink{block: make(chan struct{})}
	asyncSink := sink.NewAsyncSink(hangSink, 10)
	asyncSink.SetDrainTimeout(60 * time.Millisecond)

	// Enqueue entries into the sink
	for i := 0; i < 5; i++ {
		e := &models.LogEntry{
			Level:   models.LevelInfo,
			Message: "buffered during hang",
		}
		_ = asyncSink.Write(e)
	}

	start := time.Now()
	err := asyncSink.Close()
	elapsed := time.Since(start)

	// Close must time out gracefully and return an error without blocking indefinitely
	assert.Error(t, err, "Expected timeout error on stalled downstream sink")
	assert.True(t, strings.Contains(err.Error(), "timeout"), "Error message should mention timeout")
	assert.Less(t, elapsed, 500*time.Millisecond, "Close must unblock promptly within bounded timeout")

	// Release hanging sink channels
	close(hangSink.block)
}
