package scenarios

import (
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

	// Retry connecting to web-interface (port 8000 or 8080)
	urls := []string{"http://127.0.0.1:8000", "http://127.0.0.1:8080"}
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

	fmt.Println(">>> Web Interface Bootstrap Scenario completed successfully!")
}
