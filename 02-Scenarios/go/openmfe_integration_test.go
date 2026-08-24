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

	webURL := "http://127.0.0.1:8000"

	// 1. Check if web-interface is reachable
	resp, err := http.Get(webURL)
	if err != nil {
		t.Skipf("Skipping live integration test: web-interface on %s is offline (%v)", webURL, err)
		return
	}
	resp.Body.Close()

	// 2. Register base-scripts microservice via POST /api/v1/register
	regData := map[string]string{
		"name":     "base-scripts",
		"tag":      "base-scripts-mfe",
		"url":      "http://127.0.0.1:8085/static/mfe.js",
		"navTitle": "🚀 Squad Control",
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
		assert.Contains(t, string(bodyBytes), "base-scripts", "Registered service 'base-scripts' must appear in /api/v1/services")
	}

	fmt.Println(">>> OpenMFE Registration Scenario completed successfully!")
}
