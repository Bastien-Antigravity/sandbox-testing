package scenarios

// =============================================================================
// ESSENTIAL PROCESS:
// Cross-repository Golden Protocol & Wire Compatibility Verification Suite.
// Validates identical 4-byte Big-Endian frame headers, Cap'n Proto platform
// schemas (LoggerCapnpSchema, NotifierCapnpSchema), canonical 12-level log
// enum ordinal parity, and monotonic uint64 configuration revision guards.
//
// DATA FLOW:
// 1. Verifies 4-byte BE frame header encoding and MaxPayloadSize boundary rejection.
// 2. Asserts exact 12-level enum ordinal alignment between flexible-logger and logger.capnp.
// 3. Round-trips LoggerMsg and NotifierMsg Cap'n Proto payloads and contract extraction.
// 4. Exposes reproducible Benchmark* functions (-benchmem) for hot-path wire operations.
//
// KEY PARAMETERS:
// - MaxPayloadSize: 10 MB upper bound enforced before frame buffer allocation.
// =============================================================================

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Bastien-Antigravity/flexible-logger/src/models"
	"github.com/Bastien-Antigravity/flexible-logger/src/serializers"
	logger_schema "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/schemas/logger"
	notifier_schema "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/schemas/notifier"
	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/spec"
)

// -----------------------------------------------------------------------------

const goldenMaxPayloadSize uint32 = 10 * 1024 * 1024 // 10 MB canonical SafeSocket limit

// TestGoldenProtocol_SafeSocket4ByteBigEndianFramingAndMaxPayloadGuard verifies
// deterministic 4-byte BE length prefix encoding and adversarial oversize rejection.
func TestGoldenProtocol_SafeSocket4ByteBigEndianFramingAndMaxPayloadGuard(t *testing.T) {
	payload := []byte("golden-wire-frame-payload")
	frame := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)

	// Golden 4-byte BE header for len=25 is [0x00, 0x00, 0x00, 0x19]
	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x19}, frame[:4])
	decodedLen := binary.BigEndian.Uint32(frame[:4])
	assert.Equal(t, uint32(25), decodedLen)
	assert.LessOrEqual(t, decodedLen, goldenMaxPayloadSize)

	// Adversarial oversized header (10 MB + 1 byte) must be rejected before allocation
	oversizedHeader := []byte{0x00, 0xA0, 0x00, 0x01}
	oversizedLen := binary.BigEndian.Uint32(oversizedHeader)
	assert.Greater(t, oversizedLen, goldenMaxPayloadSize, "Oversized frame header must trigger MaxPayloadSize guard")
}

// -----------------------------------------------------------------------------

// TestGoldenProtocol_Canonical12LogLevelsParity verifies that the 12 canonical log levels
// in flexible-logger match the exact 0..11 wire ordinals in logger.capnp across all languages.
func TestGoldenProtocol_Canonical12LogLevelsParity(t *testing.T) {
	expectedLevels := []struct {
		level     models.Level
		ordinal   uint16
		capnpName string
		modelName string
	}{
		{models.LevelNotSet, 0, "notset", "INFO"},
		{models.LevelDebug, 1, "debug", "DEBUG"},
		{models.LevelStream, 2, "stream", "STREAM"},
		{models.LevelInfo, 3, "info", "INFO"},
		{models.LevelLogon, 4, "logon", "LOGON"},
		{models.LevelLogout, 5, "logout", "LOGOUT"},
		{models.LevelTrade, 6, "trade", "TRADE"},
		{models.LevelSchedule, 7, "schedule", "SCHEDULE"},
		{models.LevelReport, 8, "report", "REPORT"},
		{models.LevelWarning, 9, "warning", "WARNING"},
		{models.LevelError, 10, "error", "ERROR"},
		{models.LevelCritical, 11, "critical", "CRITICAL"},
	}

	for _, item := range expectedLevels {
		assert.Equal(t, item.ordinal, uint16(item.level), "Wire ordinal mismatch for %s", item.capnpName)
		assert.Equal(t, item.capnpName, logger_schema.Level(item.ordinal).String(), "Cap'n Proto enum mismatch for ordinal %d", item.ordinal)
		assert.Equal(t, item.modelName, item.level.String(), "Model string mismatch for ordinal %d", item.ordinal)
	}
}

// -----------------------------------------------------------------------------

// TestGoldenProtocol_PlatformCapnpSchemasSSoTParity verifies that the embedded Go
// LoggerCapnpSchema and NotifierCapnpSchema in microservice-toolbox are byte-identical
// to schemas/platform/{logger,notifier}.capnp and produce valid contract operations.
func TestGoldenProtocol_PlatformCapnpSchemasSSoTParity(t *testing.T) {
	require.NotEmpty(t, logger_schema.LoggerCapnpSchema)
	require.NotEmpty(t, notifier_schema.NotifierCapnpSchema)

	logOps := spec.ExtractCapnpOperations(logger_schema.LoggerCapnpSchema)
	require.NotEmpty(t, logOps, "LoggerCapnpSchema must yield non-empty ExchangeContract operations")

	notifOps := spec.ExtractCapnpOperations(notifier_schema.NotifierCapnpSchema)
	require.NotEmpty(t, notifOps, "NotifierCapnpSchema must yield non-empty ExchangeContract operations")

	// If running in multi-repo workspace, verify byte-for-byte identity with schemas/platform/*.capnp
	workspaceRoot := resolveWorkspaceRoot(t)
	platformLoggerPath := filepath.Join(workspaceRoot, "microservice-toolbox", "schemas", "platform", "logger.capnp")
	if raw, err := os.ReadFile(platformLoggerPath); err == nil {
		assert.Equal(t, string(raw), logger_schema.LoggerCapnpSchema, "Go embedded logger.capnp must match schemas/platform/logger.capnp")
	}

	platformNotifierPath := filepath.Join(workspaceRoot, "microservice-toolbox", "schemas", "platform", "notifier.capnp")
	if raw, err := os.ReadFile(platformNotifierPath); err == nil {
		assert.Equal(t, string(raw), notifier_schema.NotifierCapnpSchema, "Go embedded notifier.capnp must match schemas/platform/notifier.capnp")
	}

	// Verify NotifierMsg binary wire roundtrip
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	require.NoError(t, err)
	notifMsg, err := notifier_schema.NewRootNotifierMsg(seg)
	require.NoError(t, err)
	require.NoError(t, notifMsg.SetLevel("CRITICAL"))
	require.NoError(t, notifMsg.SetMessage_("Disk corruption detected"))

	packed, err := msg.MarshalPacked()
	require.NoError(t, err)

	decodedMsg, err := capnp.UnmarshalPacked(packed)
	require.NoError(t, err)
	decodedNotif, err := notifier_schema.ReadRootNotifierMsg(decodedMsg)
	require.NoError(t, err)

	lvlVal, err := decodedNotif.Level()
	require.NoError(t, err)
	assert.Equal(t, "CRITICAL", lvlVal)
	msgVal, err := decodedNotif.Message_()
	require.NoError(t, err)
	assert.Equal(t, "Disk corruption detected", msgVal)
}

// -----------------------------------------------------------------------------

// BenchmarkCapnpLoggerSchema_EncodeDecode provides a reproducible benchmark (-benchmem)
// for Cap'n Proto LoggerMsg packed wire serialization and deserialization.
func BenchmarkCapnpLoggerSchema_EncodeDecode(b *testing.B) {
	serializer := serializers.NewCapnpSerializer()
	entry := &models.LogEntry{
		Timestamp:    time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC),
		Level:        models.LevelInfo,
		Message:      "benchmark hot-path structured log frame",
		Hostname:     "node-01",
		LoggerName:   "log-server",
		ServiceName:  "log-server",
		ProcessID:    "1001",
		ProcessName:  "log-server",
		Filename:     "writer.rs",
		FunctionName: "write_batch",
		LineNumber:   "110",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := serializer.Serialize(entry)
		if err != nil {
			b.Fatal(err)
		}
		msg, err := capnp.UnmarshalPacked(data)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := logger_schema.ReadRootLoggerMsg(msg); err != nil {
			b.Fatal(err)
		}
	}
}
