package scenarios_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"ontime-scheduler/src/api"
	"ontime-scheduler/src/models"
	"ontime-scheduler/src/scheduler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSchedulerFullLifecycle(t *testing.T) {
	// 1. Setup Database
	dbPath := "integration_test.db"
	defer os.Remove(dbPath)
	os.MkdirAll("scheduled", 0755)
	defer os.RemoveAll("scheduled")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	db.AutoMigrate(&models.Job{}, &models.JobLog{})

	// 2. Setup Engine & Server
	engine := scheduler.NewEngine(db, nil)
	engine.Start()
	defer engine.Stop()

	server := api.NewServer(engine, db)
	go server.Run("127.0.0.1:8181")
	time.Sleep(500 * time.Millisecond) // Wait for server to start

	// 3. Scenario: Create Job via Legacy API
	scriptContent := `
def integration_task():
    print("HELLO_FROM_SANDBOX")
`
	payload := map[string]interface{}{
		"name": "Sandbox Integration Job",
		"func": "test_module:integration_task",
		"trigger": "cron",
		"cron": "* * * * *",
		"script": scriptContent,
		"max_instances": 1,
	}
	jsonPayload, _ := json.Marshal(payload)

	resp, err := http.Post("http://127.0.0.1:8181/create_job", "application/json", bytes.NewBuffer(jsonPayload))
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// 4. Verify Persistence
	var job models.Job
	err = db.First(&job, "name = ?", "Sandbox Integration Job").Error
	assert.NoError(t, err)
	assert.Equal(t, "cron", job.TriggerType)
	assert.NotEmpty(t, job.ScriptPath)

	// 5. Scenario: Manual Execution
	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:8181/exec_job/%s", job.ID), "application/json", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 6. Verify Execution Log
	time.Sleep(1 * time.Second) // Wait for async execution
	var logEntry models.JobLog
	err = db.First(&logEntry, "job_id = ?", job.ID).Error
	assert.NoError(t, err)
	assert.Contains(t, logEntry.LogMessage, "executed successfully")
}
