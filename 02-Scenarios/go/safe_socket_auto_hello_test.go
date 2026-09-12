package scenarios

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Bastien-Antigravity/safe-socket/src/facade"
	"github.com/Bastien-Antigravity/safe-socket/src/factory"
	"github.com/Bastien-Antigravity/safe-socket/src/interfaces"
	"github.com/Bastien-Antigravity/safe-socket/src/models"
	"github.com/Bastien-Antigravity/safe-socket/src/profiles"
	"github.com/Bastien-Antigravity/safe-socket/src/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// UNIT TESTS: Auto-Hello Dynamic Resolution & Machine Detection
// =============================================================================

// TestAutoHello_IsCompletelyDynamic verifies that "auto-hello" requires NO extra
// protocol or transport parameters. It dynamically decides between plain TCP and
// TLS purely based on the target IP/host at runtime.
func TestAutoHello_IsCompletelyDynamic(t *testing.T) {
	detector := utils.GetMachineDetector()

	// 1. Local targets must dynamically resolve to local (no TLS needed)
	localTargets := []string{
		"127.0.0.1:9020",
		"127.0.0.2:3306",
		"localhost:1026",
		"0.0.0.0:5000",
		"[::1]:8080",
	}
	for _, target := range localTargets {
		assert.True(t, detector.IsLocalAddress(target), "Expected %s to be recognized as local", target)
	}

	// 2. Remote targets must dynamically resolve to remote (TLS needed)
	remoteTargets := []string{
		"198.51.100.25:9020",
		"203.0.113.10:3306",
		"remote.bastien-antigravity.internal:4222",
	}
	for _, target := range remoteTargets {
		assert.False(t, detector.IsLocalAddress(target), "Expected %s to be recognized as remote", target)
	}
}

// TestAutoHelloProfile_LocalResolution verifies that creating an "auto-hello"
// client for a local address automatically selects the unencrypted TcpHelloClientProfile.
func TestAutoHelloProfile_LocalResolution(t *testing.T) {
	clientSocket, err := factory.CreateWithConfig(
		"auto-hello:test-client",
		"127.0.0.1:9020",
		models.SocketConfig{
			ServiceAddress: "127.0.0.1:1863",
		},
		"client",
		false, // do not auto-connect
	)
	require.NoError(t, err)
	require.NotNil(t, clientSocket)

	client, ok := clientSocket.(*facade.SocketClient)
	require.True(t, ok, "Expected clientSocket to be *facade.SocketClient")

	// Verify that the underlying profile is TcpHelloClientProfile (unencrypted)
	profile := client.Profile
	require.NotNil(t, profile)
	_, isTcpHello := profile.(*profiles.TcpHelloClientProfile)
	assert.True(t, isTcpHello, "Expected profile to be *profiles.TcpHelloClientProfile for local target, got %T", profile)
	assert.Equal(t, interfaces.TransportFramedTCP, profile.GetTransport())
	assert.Equal(t, interfaces.ProtocolHello, profile.GetProtocol())
	assert.Equal(t, "test-client", profile.GetName())
}

// TestAutoHelloProfile_RemoteResolution verifies that creating an "auto-hello"
// client for a remote address automatically selects the encrypted TlsHelloClientProfile.
func TestAutoHelloProfile_RemoteResolution(t *testing.T) {
	clientSocket, err := factory.CreateWithConfig(
		"auto-hello:test-remote-client",
		"198.51.100.50:9020",
		models.SocketConfig{
			ServiceAddress: "198.51.100.1:1863",
		},
		"client",
		false, // do not auto-connect
	)
	require.NoError(t, err)
	require.NotNil(t, clientSocket)

	client, ok := clientSocket.(*facade.SocketClient)
	require.True(t, ok, "Expected clientSocket to be *facade.SocketClient")

	// Verify that the underlying profile is TlsHelloClientProfile (encrypted)
	profile := client.Profile
	require.NotNil(t, profile)
	_, isTlsHello := profile.(*profiles.TlsHelloClientProfile)
	assert.True(t, isTlsHello, "Expected profile to be *profiles.TlsHelloClientProfile for remote target, got %T", profile)
	assert.Equal(t, interfaces.TransportTLS, profile.GetTransport())
	assert.Equal(t, interfaces.ProtocolHello, profile.GetProtocol())
	assert.Equal(t, "test-remote-client", profile.GetName())
}

// TestAutoHello_ServiceAddressConfig verifies that models.SocketConfig.ServiceAddress
// is retained and accessible on the created socket for dynamic discovery.
func TestAutoHello_ServiceAddressConfig(t *testing.T) {
	advertisedAddr := "127.0.0.2:8085"
	cfg := models.SocketConfig{
		ServiceAddress: advertisedAddr,
	}

	sock, err := factory.CreateWithConfig("auto-hello:discovery-test", "127.0.0.1:9090", cfg, "client", false)
	require.NoError(t, err)
	require.NotNil(t, sock)

	client, ok := sock.(*facade.SocketClient)
	require.True(t, ok, "Expected sock to be *facade.SocketClient")
	assert.Equal(t, advertisedAddr, client.Config.ServiceAddress, "ServiceAddress should be preserved in socket config")
}

// =============================================================================
// INTEGRATION TESTS: End-to-End Handshake with Auto-Hello
// =============================================================================

// TestAutoHello_LocalEndToEnd_Communication tests a complete real-world communication
// using "auto-hello":
// 1. Starts a SafeSocket server on an ephemeral local port using "auto-hello".
// 2. Starts a SafeSocket client using "auto-hello" with an advertised ServiceAddress.
// 3. Verifies successful connection, handshake, and bidirectional message delivery.
func TestAutoHello_LocalEndToEnd_Communication(t *testing.T) {
	// 1. Pick a free local port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serverAddr := ln.Addr().String()
	ln.Close() // Release so SafeSocket can bind

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. Start Server using auto-hello
	serverSocket, err := factory.CreateWithConfig(
		"auto-hello:test-service-server",
		serverAddr,
		models.SocketConfig{
			ServiceAddress: serverAddr,
		},
		"server",
		true, // Listen immediately
	)
	require.NoError(t, err, "Server failed to start with auto-hello")
	defer serverSocket.Close()

	// Server accept loop in background
	serverReceived := make(chan string, 1)
	go func() {
		for {
			conn, err := serverSocket.Accept()
			if err != nil {
				return
			}
			go func(c interfaces.TransportConnection) {
				defer c.Close()
				data, err := c.ReadMessage()
				if err == nil && len(data) > 0 {
					serverReceived <- string(data)
					// Echo back response (FramedTCPSocket.Write automatically frames with 4-byte length)
					resp := []byte("PONG:" + string(data))
					_, _ = c.Write(resp)
				}
			}(conn)
		}
	}()

	// 3. Start Client using auto-hello pointing to local server
	clientSocket, err := factory.CreateWithConfig(
		"auto-hello:test-service-client",
		serverAddr,
		models.SocketConfig{
			ServiceAddress: "127.0.0.1:55555", // Advertised reachable callback address
		},
		"client",
		true, // Open/Connect immediately
	)
	require.NoError(t, err, "Client failed to connect with auto-hello")
	defer clientSocket.Close()

	// 4. Send a message from client to server
	testPayload := fmt.Sprintf("HELLO-AUTO-TEST-%d", time.Now().UnixNano())
	err = clientSocket.Send([]byte(testPayload))
	require.NoError(t, err, "Client failed to send payload")

	// 5. Verify server received the payload
	select {
	case received := <-serverReceived:
		assert.Equal(t, testPayload, received, "Server should receive the exact payload sent by client")
	case <-ctx.Done():
		t.Fatal("Timeout waiting for server to receive payload over auto-hello connection")
	}

	// 6. Verify client receives echo response
	reply, err := clientSocket.Receive()
	require.NoError(t, err, "Client failed to receive response")
	assert.Equal(t, "PONG:"+testPayload, string(reply))
}
