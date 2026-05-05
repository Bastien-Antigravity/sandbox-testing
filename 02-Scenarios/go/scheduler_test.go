package go_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegacyPythonExecution(t *testing.T) {
	// 1. Prepare the command
	// Relative path from sandbox-testing/implementations/go
	scriptPath := "../../../ontime-scheduler/go/scripts/test_legacy_job.py"
	
	cmd := exec.Command("python3", scriptPath, "arg1", "arg2")
	
	// 2. Execute
	output, err := cmd.CombinedOutput()
	
	// 3. Verify
	assert.NoError(t, err, "Python script should execute without error")
	assert.Contains(t, string(output), "Legacy Python job executed successfully!", "Output should contain success message")
	assert.Contains(t, string(output), "['arg1', 'arg2']", "Output should contain passed arguments")
}
