package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSquadControlIntegrationScenario(t *testing.T) {
	fmt.Println(">>> Scenario Test: Verifying Squad Control REST API & History Logs")

	squadURL := "http://127.0.0.1:8085"

	// 1. Check if Base-Scripts daemon (Port 8085) is reachable
	resp, err := http.Get(squadURL + "/api/v1/squad/chat/test_scenario_session")
	if err != nil {
		t.Skipf("Skipping live integration test: Base-Scripts daemon on %s is offline (%v)", squadURL, err)
		return
	}
	resp.Body.Close()

	// 2. Post a chat message to session
	msgData := map[string]string{
		"message": "@orchestrator Scenario test message verification",
	}
	payloadBytes, _ := json.Marshal(msgData)

	postResp, err := http.Post(squadURL+"/api/v1/squad/chat/test_scenario_session", "application/json", bytes.NewBuffer(payloadBytes))
	assert.NoError(t, err)
	if postResp != nil {
		assert.Equal(t, http.StatusOK, postResp.StatusCode, "Post message should return status 200 OK")
		postResp.Body.Close()
	}

	// 3. Retrieve chat history
	getResp, err := http.Get(squadURL + "/api/v1/squad/chat/test_scenario_session")
	assert.NoError(t, err)
	if getResp != nil {
		defer getResp.Body.Close()
		bodyBytes, _ := io.ReadAll(getResp.Body)
		assert.Contains(t, string(bodyBytes), "Scenario test message verification", "History must contain posted message")
	}

	fmt.Println(">>> Squad Control Scenario completed successfully!")
}
