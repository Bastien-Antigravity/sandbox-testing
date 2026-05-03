package scenarios

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigServerResilience(t *testing.T) {
	// 1. Setup: Ensure we are in a clean state (Docker CLI accessible)
	_, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("Docker CLI not found, skipping integration test")
	}

	fmt.Println(">>> Starting Config Server Resilience Scenario")

	// 2. Scenario: Stop the Config Server
	fmt.Println(">>> Stopping sandbox-config-server...")
	cmd := exec.Command("docker", "stop", "sandbox-config-server")
	err = cmd.Run()
	assert.NoError(t, err, "Should be able to stop config-server")

	// 3. Wait and observe client behavior (e.g., notif-server)
	// We expect some logs indicating reconnection attempts or failure to sync
	fmt.Println(">>> Waiting 5 seconds for clients to detect outage...")
	time.Sleep(5 * time.Second)

	// Check logs of notif-server
	logCmd := exec.Command("docker", "logs", "--tail", "20", "sandbox-notif-server")
	var out bytes.Buffer
	logCmd.Stdout = &out
	_ = logCmd.Run()

	logs := out.String()
	fmt.Printf(">>> Notif-Server Logs during outage:\n%s\n", logs)
	// We don't assert specific strings yet as we want to observe first, 
	// but we expect to see some network-related errors.

	// 4. Scenario: Restart the Config Server
	fmt.Println(">>> Restarting sandbox-config-server...")
	cmd = exec.Command("docker", "start", "sandbox-config-server")
	err = cmd.Run()
	assert.NoError(t, err, "Should be able to restart config-server")

	// 5. Verify recovery
	fmt.Println(">>> Waiting 5 seconds for recovery...")
	time.Sleep(5 * time.Second)

	// Check if clients re-identified
	logCmd = exec.Command("docker", "logs", "--tail", "20", "sandbox-config-server")
	out.Reset()
	logCmd.Stdout = &out
	_ = logCmd.Run()
	
	srvLogs := out.String()
	fmt.Printf(">>> Config-Server Logs after recovery:\n%s\n", srvLogs)

	// Assert that at least one client re-identified
	assert.Contains(t, srvLogs, "Client identified", "At least one client should have reconnected")

	fmt.Println(">>> Config Server Resilience Scenario Completed")
}
