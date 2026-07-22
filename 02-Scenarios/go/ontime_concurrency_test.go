package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ontimeBaseURL = "http://localhost:8080"

// TestOntimeConcurrentJobCreation validates that the scheduler can handle
// 50 simultaneous job creation requests without data corruption.
func TestOntimeConcurrentJobCreation(t *testing.T) {
	if !waitForOntimeServer(t) {
		t.Fatal("Server not available at", ontimeBaseURL)
	}
	setOntimeDistributedMode(t)

	var successCount int32
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobReq := map[string]interface{}{
				"id":            fmt.Sprintf("concurrent-job-sleep-%d", idx),
				"name":          fmt.Sprintf("Concurrent Sleep %d", idx),
				"trigger":       "Exec",
				"kwargs":        "sleep 2",
				"max_retries":   1,
				"max_instances": 1,
			}
			body, _ := json.Marshal(jobReq)

			resp, err := http.Post(ontimeBaseURL+"/jobs", "application/json", bytes.NewBuffer(body))
			if err == nil && resp.StatusCode == http.StatusCreated {
				atomic.AddInt32(&successCount, 1)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Created %d/50 jobs in %v", successCount, time.Since(start))
	assert.Equal(t, int32(50), successCount, "All 50 concurrent job creations should succeed")
}

// TestOntimeConcurrentExecutionLock validates that the scheduler's concurrency
// lock prevents multiple instances of the same job from running simultaneously.
// It fires 50 simultaneous execute requests against a job with max_instances=1
// and verifies that only 1 execution actually occurred.
func TestOntimeConcurrentExecutionLock(t *testing.T) {
	if !waitForOntimeServer(t) {
		t.Fatal("Server not available at", ontimeBaseURL)
	}
	setOntimeDistributedMode(t)

	// Ensure the target job exists
	jobReq := map[string]interface{}{
		"id":            "concurrent-job-sleep-0",
		"name":          "Concurrent Sleep 0",
		"trigger":       "Exec",
		"kwargs":        "sleep 2",
		"max_retries":   1,
		"max_instances": 1,
	}
	body, _ := json.Marshal(jobReq)
	resp, err := http.Post(ontimeBaseURL+"/jobs", "application/json", bytes.NewBuffer(body))
	if resp != nil {
		resp.Body.Close()
	}
	require.NoError(t, err)

	// Fire 50 concurrent execution triggers
	var execSuccessCount int32
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("POST", ontimeBaseURL+"/jobs/concurrent-job-sleep-0/execute", nil)
			resp, err := http.DefaultClient.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				atomic.AddInt32(&execSuccessCount, 1)
			}
			if resp != nil {
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	t.Logf("Submitted %d/50 execution requests in %v", execSuccessCount, time.Since(start))

	// Wait for execution logs to flush to DB
	time.Sleep(4 * time.Second)

	// Verify that only 1 execution actually ran
	resp, err = http.Get(ontimeBaseURL + "/infos_job/concurrent-job-sleep-0")
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var logResp struct {
		Logs []struct {
			LogType    string `json:"log_type"`
			LogMessage string `json:"log_message"`
		} `json:"logs"`
	}
	json.Unmarshal(bodyBytes, &logResp)

	execs := 0
	for _, l := range logResp.Logs {
		if l.LogType == "INFO" && strings.Contains(l.LogMessage, "successfully") {
			execs++
		}
	}

	t.Logf("Successful runs recorded: %d (Expected: 1)", execs)
	assert.Equal(t, 1, execs, "Concurrency lock should ensure only 1 instance ran despite 50 triggers")
}

// --- Helpers ---

func waitForOntimeServer(t *testing.T) bool {
	t.Helper()
	for i := 0; i < 10; i++ {
		resp, err := http.Get(ontimeBaseURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func setOntimeDistributedMode(t *testing.T) {
	t.Helper()
	body := []byte(`{"distributed": true}`)
	req, _ := http.NewRequest("POST", ontimeBaseURL+"/scheduler/mode", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		t.Log("Engine mode set to: DISTRIBUTED")
		resp.Body.Close()
	}
}
