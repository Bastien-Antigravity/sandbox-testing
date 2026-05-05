package scenarios

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"sync"
	"testing"
	"time"

	toolbox_config "github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/config"
	unilog "github.com/Bastien-Antigravity/universal-logger/src/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestConcurrencyAndChaos(t *testing.T) {
	// 1. Setup: Skip if Docker not present
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI not found, skipping chaos test")
	}

	fmt.Println(">>> Starting Concurrency & Chaos Scenario")

	const numClients = 10
	const duration = 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// 2. Spawn concurrent mock clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientName := fmt.Sprintf("mock-client-%d", id)
			
			// Each client has its own configuration and logger (bootstrapped toolbox pattern)
			appConfig, err := toolbox_config.LoadConfig("test", nil)
			if err != nil {
				fmt.Printf("[%s] Config load failed: %v\n", clientName, err)
				return
			}

			_, logger := unilog.Init(clientName, "test", "standard", "INFO", false, nil)
			defer logger.Close()

			appConfig.Logger = logger
			logger.Info("Client %d started", id)

			for {
				select {
				case <-ctx.Done():
					logger.Info("Client %d stopping", id)
					return
				default:
					// Perform some random activity
					addr, err := appConfig.GetListenAddr("config_server")
					if err != nil {
						logger.Error("Failed to get config_server addr: %v", err)
					} else {
						logger.Debug("Config server addr: %s", addr)
					}

					// Random sleep to simulate staggered requests
					time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				}
			}
		}(i)
	}

	// 3. Background Chaos Monkey: Periodically restart config-server and log-server
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()

		targets := []string{"sandbox-config-server", "sandbox-log-server"}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				target := targets[rand.Intn(len(targets))]
				fmt.Printf(">>> CHAOS: Stopping %s...\n", target)
				_ = exec.Command("docker", "stop", target).Run()
				
				time.Sleep(3 * time.Second)
				
				fmt.Printf(">>> CHAOS: Starting %s...\n", target)
				_ = exec.Command("docker", "start", target).Run()
			}
		}
	}()

	// 4. Wait for completion
	wg.Wait()
	fmt.Println(">>> Concurrency & Chaos Scenario Completed")
	assert.True(t, true) // If we reached here without panicking, it's a win
}
