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

func TestOpenMFERegistrationScenario(t *testing.T) {
	fmt.Println(">>> Scenario Test: Verifying OpenMFE Dynamic Microservice Registration & Routing")

	webURL := "http://127.0.0.1:5000"

	// 1. Check if web-interface is reachable
	resp, err := http.Get(webURL)
	if err != nil {
		t.Skipf("Skipping live integration test: web-interface on %s is offline (%v)", webURL, err)
		return
	}
	resp.Body.Close()

	// 2. Register squad-agent microservice via POST /api/v1/register (with ExchangeContracts)
	regData := map[string]interface{}{
		"name":     "squad-agent",
		"tag":      "squad-agent-mfe",
		"url":      "http://127.0.0.1:8085/static/mfe.js",
		"navTitle": "🚀 Squad Control",
		"layer":    "L3",
		"category": "Secure Controls",
		"contracts": []map[string]interface{}{
			{
				"id":        "rest-sse",
				"type":      "HTTP/REST & SSE API",
				"tabLabel":  "HTTP/REST & SSE",
				"direction": "inbound",
			},
		},
	}
	payloadBytes, _ := json.Marshal(regData)

	regResp, err := http.Post(webURL+"/api/v1/register", "application/json", bytes.NewBuffer(payloadBytes))
	assert.NoError(t, err, "Failed to POST /api/v1/register")
	if regResp != nil {
		assert.True(t, regResp.StatusCode == http.StatusCreated || regResp.StatusCode == http.StatusOK, "Register endpoint should return status 201 Created or 200 OK")
		regResp.Body.Close()
	}

	// 3. Query GET /api/v1/services to verify registration persistence
	servicesResp, err := http.Get(webURL + "/api/v1/services")
	assert.NoError(t, err)
	if servicesResp != nil {
		defer servicesResp.Body.Close()
		bodyBytes, _ := io.ReadAll(servicesResp.Body)
		assert.Contains(t, string(bodyBytes), "squad-agent", "Registered service 'squad-agent' must appear in /api/v1/services")
	}

	// 4. Query GET /api/v1/contracts?service=squad-agent to verify published contracts
	contractsResp, err := http.Get(webURL + "/api/v1/contracts?service=squad-agent")
	assert.NoError(t, err)
	if contractsResp != nil {
		defer contractsResp.Body.Close()
		assert.Equal(t, http.StatusOK, contractsResp.StatusCode)
		bodyBytes, _ := io.ReadAll(contractsResp.Body)
		assert.Contains(t, string(bodyBytes), "rest-sse", "Published ExchangeContract must be served by /api/v1/contracts")
	}

	fmt.Println(">>> OpenMFE Registration Scenario completed successfully!")
}
