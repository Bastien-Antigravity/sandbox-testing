package scenarios

// =============================================================================
// ESSENTIAL PROCESS: Integration scenario validating dynamic log level filtering and alert notification dispatching across services.
//
// DATA FLOW:
//   1. Initializes microservice logging context via universal-logger bootstrap facade.
//   2. Emits structured log events while dynamically modifying log level thresholds via distributed-config.
//   3. Validates that sub-threshold events are dropped and above-threshold events are captured.
//   4. Asserts that high-severity events (Warning, Error, Critical) trigger alert notifications on subscriber channels.
//
// KEY PARAMETERS:
//   - TestDynamicLoggingAndAlertsScenario: End-to-end multi-level and notification validation.
// =============================================================================

import (
	"testing"
	"time"

	unilog "github.com/Bastien-Antigravity/universal-logger/src/bootstrap"
	"github.com/Bastien-Antigravity/universal-logger/src/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------

func TestDynamicLoggingAndAlertsScenario(t *testing.T) {
	// 1. Initialize service with standalone profile and local notifier enabled
	distConfig, uniLog := unilog.Init("sandbox-logger-svc", "standalone", "devel", "INFO", true, nil)
	require.NotNil(t, distConfig, "Distributed config should initialize")
	require.NotNil(t, uniLog, "UniLog instance should initialize")
	defer uniLog.Close()

	notifQueue := uniLog.GetNotifQueue()
	require.NotNil(t, notifQueue, "Local notification queue must be present when useLocalNotifier is true")

	// -------------------------------------------------------------------------
	// Phase 1: Dynamic Log Level Filtering & Synchronization
	// -------------------------------------------------------------------------
	t.Run("Dynamic_Log_Level_Transitions", func(t *testing.T) {
		assert.Equal(t, utils.LevelInfo, uniLog.GetLevel(), "Initial log level should be INFO")

		// Elevate to WARNING
		err := distConfig.SetConfig("logger", "level", "WARNING")
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, utils.LevelWarning, uniLog.GetLevel(), "Level should synchronize to WARNING")

		// Elevate to ERROR
		err = distConfig.SetConfig("logger", "level", "ERROR")
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, utils.LevelError, uniLog.GetLevel(), "Level should synchronize to ERROR")

		// Lower to DEBUG
		err = distConfig.SetConfig("logger", "level", "DEBUG")
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, utils.LevelDebug, uniLog.GetLevel(), "Level should synchronize to DEBUG")

		// Direct programmatic adjustment
		uniLog.SetLevel(utils.LevelInfo)
		assert.Equal(t, utils.LevelInfo, uniLog.GetLevel(), "Direct SetLevel should update level to INFO")
	})

	// -------------------------------------------------------------------------
	// Phase 2: Notification Alert Severity Threshold Gating
	// -------------------------------------------------------------------------
	t.Run("Alert_Threshold_Gating", func(t *testing.T) {
		// Low-severity and operational domain logs must never enter the notification queue
		uniLog.Debug("sandbox-debug-msg")
		uniLog.Info("sandbox-info-msg")
		uniLog.Stream("sandbox-stream-msg")
		uniLog.Logon("sandbox-logon-msg")
		uniLog.Logout("sandbox-logout-msg")
		uniLog.Trade("sandbox-trade-msg")
		uniLog.Schedule("sandbox-schedule-msg")
		uniLog.Report("sandbox-report-msg")

		select {
		case msg := <-notifQueue:
			t.Fatalf("Low severity log incorrectly triggered notification: %+v", msg)
		case <-time.After(100 * time.Millisecond):
			// Success: queue remained clean
		}

		// WARNING severity alert
		uniLog.Warning("sandbox-warning-alert")
		select {
		case msg := <-notifQueue:
			assert.Equal(t, "WARNING", msg.Level)
			assert.Equal(t, "sandbox-warning-alert", msg.Message)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for WARNING alert on notifQueue")
		}

		// ERROR severity alert
		uniLog.Error("sandbox-error-alert")
		select {
		case msg := <-notifQueue:
			assert.Equal(t, "ERROR", msg.Level)
			assert.Equal(t, "sandbox-error-alert", msg.Message)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for ERROR alert on notifQueue")
		}

		// CRITICAL severity alert
		uniLog.Critical("sandbox-critical-alert")
		select {
		case msg := <-notifQueue:
			assert.Equal(t, "CRITICAL", msg.Level)
			assert.Equal(t, "sandbox-critical-alert", msg.Message)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for CRITICAL alert on notifQueue")
		}
	})

	// -------------------------------------------------------------------------
	// Phase 3: Caller Metadata and Polyglot Alert Preservation
	// -------------------------------------------------------------------------
	t.Run("Caller_Metadata_Alert_Preservation", func(t *testing.T) {
		// LogWithCaller alert
		uniLog.LogWithCaller(utils.LevelWarning, "caller-alert-payload", "service.py", "45", "handle_task", "worker.service")
		select {
		case msg := <-notifQueue:
			assert.Equal(t, "WARNING", msg.Level)
			assert.Equal(t, "caller-alert-payload", msg.Message)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for LogWithCaller alert")
		}

		// LogWithMetadata alert
		utils.LogWithMetadata(uniLog, utils.LevelError, "meta-alert-payload", "engine.rs", "102", "execute", "engine")
		select {
		case msg := <-notifQueue:
			assert.Equal(t, "ERROR", msg.Level)
			assert.Equal(t, "meta-alert-payload", msg.Message)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for LogWithMetadata alert")
		}
	})

	// -------------------------------------------------------------------------
	// Phase 4: Non-Blocking Buffer Resilience Under Saturation
	// -------------------------------------------------------------------------
	t.Run("Non_Blocking_Saturation_Resilience", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			for i := 0; i < 1100; i++ {
				uniLog.Warning("saturation-test-%d", i)
			}
			close(done)
		}()

		select {
		case <-done:
			// Completed without deadlock
		case <-time.After(2 * time.Second):
			t.Fatal("Saturated notification queue caused caller to deadlock!")
		}

		// Drain the buffer and verify recovery
		drained := 0
		for {
			select {
			case <-notifQueue:
				drained++
			default:
				goto DRAIN_COMPLETE
			}
		}
	DRAIN_COMPLETE:
		assert.Equal(t, 1024, drained, "Buffer should have held maximum queue capacity")

		// Verify logger resumes normal notification dispatch after drain
		uniLog.Warning("post-saturation-recovered")
		select {
		case msg := <-notifQueue:
			assert.Equal(t, "post-saturation-recovered", msg.Message)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for post-saturation notification")
		}
	})
}
