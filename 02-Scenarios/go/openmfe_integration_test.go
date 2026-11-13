package scenarios

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOpenMFERegistrationScenario(t *testing.T) {
	fmt.Println(">>> Scenario Test: Verifying OpenMFE Dynamic Microservice Registration & Routing")

	webURL := "http://127.0.0.1:5000"
	client := &http.Client{Timeout: 2 * time.Second}

	// 1. Check if web-interface is reachable
	resp, err := client.Get(webURL)
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

	regResp, err := client.Post(webURL+"/api/v1/register", "application/json", bytes.NewBuffer(payloadBytes))
	assert.NoError(t, err, "Failed to POST /api/v1/register")
	if regResp != nil {
		assert.True(t, regResp.StatusCode == http.StatusCreated || regResp.StatusCode == http.StatusOK, "Register endpoint should return status 201 Created or 200 OK")
		regResp.Body.Close()
	}

	// 3. Query GET /api/v1/services to verify registration persistence
	servicesResp, err := client.Get(webURL + "/api/v1/services")
	assert.NoError(t, err)
	var registered []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if servicesResp != nil {
		defer servicesResp.Body.Close()
		bodyBytes, _ := io.ReadAll(servicesResp.Body)
		assert.Contains(t, string(bodyBytes), "squad-agent", "Registered service 'squad-agent' must appear in /api/v1/services")
		_ = json.Unmarshal(bodyBytes, &registered)
	}

	// 4. Query GET /api/v1/contracts?service=squad-agent to verify published contracts on web-interface
	contractsResp, err := client.Get(webURL + "/api/v1/contracts?service=squad-agent")
	assert.NoError(t, err)
	if contractsResp != nil {
		defer contractsResp.Body.Close()
		assert.Equal(t, http.StatusOK, contractsResp.StatusCode)
		bodyBytes, _ := io.ReadAll(contractsResp.Body)
		assert.Contains(t, string(bodyBytes), "rest-sse", "Published ExchangeContract must be served by /api/v1/contracts")
	}

	// 5. Probe live HTTP microservices discovered in /api/v1/services directly at their /api/v1/contracts
	for _, svc := range registered {
		if !strings.HasPrefix(svc.URL, "http://") && !strings.HasPrefix(svc.URL, "https://") {
			continue
		}
		u, parseErr := url.Parse(svc.URL)
		if parseErr != nil || u.Host == "" {
			continue
		}
		liveURL := fmt.Sprintf("%s://%s/api/v1/contracts", u.Scheme, u.Host)
		liveResp, liveErr := client.Get(liveURL)
		if liveErr != nil {
			continue // Offline optional service
		}
		var env struct {
			Reliable  bool          `json:"reliable"`
			Contracts []interface{} `json:"contracts"`
		}
		decErr := json.NewDecoder(liveResp.Body).Decode(&env)
		liveResp.Body.Close()
		if liveResp.StatusCode == http.StatusOK && decErr == nil {
			assert.True(t, env.Reliable, "Online microservice %s /api/v1/contracts must report reliable=true", svc.Name)
			assert.NotEmpty(t, env.Contracts, "Online microservice %s /api/v1/contracts must not be empty", svc.Name)
		}
	}

	fmt.Println(">>> OpenMFE Registration Scenario completed successfully!")
}
