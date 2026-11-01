package scenarios

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWebInterfaceBootstrap(t *testing.T) {
	fmt.Println(">>> Scenario Test: Verifying Web Interface Bootstrap & Configuration Injection")

	// Retry connecting to web-interface (canonical port 5000)
	urls := []string{"http://127.0.0.1:5000", "http://127.0.0.1:8000"}
	var resp *http.Response
	var err error

	for _, url := range urls {
		for i := 0; i < 5; i++ {
			resp, err = http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(500 * time.Millisecond)
		}
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			break
		}
	}

	assert.NoError(t, err, "Failed to connect to web-interface")
	if err != nil || resp == nil {
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	body := string(bodyBytes)

	// 1. Verify that the response code is 200
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Response status code should be 200 OK")

	// 2. Verify config injection: BaseURL and WssURL variables should exist in the JS section
	assert.Contains(t, body, "const SITE_BASE_URL =", "HTML response must define const SITE_BASE_URL")
	assert.Contains(t, body, "const SITE_WSS_URL =", "HTML response must define const SITE_WSS_URL")

	// 3. Verify localized asset paths exist in base.html output
	assert.True(t, strings.Contains(body, "w3.css") || strings.Contains(body, "design-tokens.css"), "CSS assets must point to local path")
	assert.Contains(t, body, "src=\"/static/js/jquery.min.js\"", "JS jquery must point to local path")

	// 4. Verify Service Inspector body and all 10 data locations (//*[@id="service-inspector"]/div[3])
	assert.Contains(t, body, "id=\"service-inspector\"", "Home page must contain #service-inspector")
	assert.Contains(t, body, "class=\"service-inspector__body\"", "Home page must contain .service-inspector__body")

	requiredInspectorTargets := []string{
		"inspector-desc",
		"inspector-cap",
		"inspector-addr",
		"inspector-protocol",
		"inspector-framing",
		"inspector-status",
		"inspector-sdks",
		"inspector-data-code",
		"inspector-flow-list",
		"inspector-sdk-code",
	}
	for _, target := range requiredInspectorTargets {
		assert.Contains(t, body, fmt.Sprintf("id=\"%s\"", target), "Inspector body must contain target #"+target)
	}

	// 5. Verify no undefined variable or function references
	assert.NotContains(t, body, "DEFAULT_SERVICE_POSITIONS", "Home page must NOT contain deprecated DEFAULT_SERVICE_POSITIONS")
	assert.NotContains(t, body, "renderTransportMesh", "Home page must NOT contain undefined renderTransportMesh")

	// 6. Verify Dynamic Fleet Status API (/api/v1/system/fleet-status)
	fleetResp, err := http.Get("http://127.0.0.1:5000/api/v1/system/fleet-status")
	assert.NoError(t, err, "Failed to call /api/v1/system/fleet-status")
	if err == nil && fleetResp != nil {
		defer fleetResp.Body.Close()
		assert.Equal(t, http.StatusOK, fleetResp.StatusCode, "Fleet status API must return 200 OK")

		var fleetData struct {
			AllOnline bool `json:"all_online"`
			Total     int  `json:"total"`
			Services  []struct {
				Name    string `json:"name"`
				Cap     string `json:"cap"`
				Address string `json:"address"`
				Online  bool   `json:"online"`
			} `json:"services"`
		}

		err = json.NewDecoder(fleetResp.Body).Decode(&fleetData)
		assert.NoError(t, err, "Fleet status response must be valid JSON")
		assert.GreaterOrEqual(t, fleetData.Total, 10, "Fleet status must report at least 10 monitored services")

		// Verify that reported addresses are dynamic and non-empty
		for _, s := range fleetData.Services {
			assert.NotEmpty(t, s.Address, "Service %s must have a non-empty dynamic address", s.Cap)
			assert.Contains(t, s.Address, ":", "Service %s address must include a valid port", s.Cap)
		}
	}

	fmt.Println(">>> Web Interface Bootstrap Scenario completed successfully!")
}
