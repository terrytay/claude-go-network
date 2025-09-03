package main

import (
	"testing"
	"time"
)

// Chapter 1: Your First Linux Socket
// Following the "Learn Go with Tests" methodology

func TestLinuxSocketCreation(t *testing.T) {
	// This test defines what we want: create a raw Linux UDP socket
	socket, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Expected to create socket, got error: %v", err)
	}
	defer socket.Close()

	// We should have a valid file descriptor
	if socket.GetFD() <= 0 {
		t.Errorf("Expected positive file descriptor, got %d", socket.GetFD())
	}
}

func TestLinuxSocketBinding(t *testing.T) {
	// Test that we can bind to a specific address and port
	socket, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Failed to create socket: %v", err)
	}
	defer socket.Close()

	// Bind to localhost:0 (let OS choose port)
	err = socket.Bind("127.0.0.1", 0)
	if err != nil {
		t.Fatalf("Expected successful bind, got error: %v", err)
	}

	// Should be able to get the bound address
	addr := socket.GetLocalAddr()
	if addr.IP != "127.0.0.1" {
		t.Errorf("Expected IP 127.0.0.1, got %s", addr.IP)
	}
	if addr.Port == 0 {
		t.Errorf("Expected OS to assign a port, got 0")
	}
}

func TestLinuxSocketSendReceive(t *testing.T) {
	// Test the core functionality: sending and receiving data
	
	// Create server socket
	server, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Failed to create server socket: %v", err)
	}
	defer server.Close()

	err = server.Bind("127.0.0.1", 0)
	if err != nil {
		t.Fatalf("Failed to bind server: %v", err)
	}

	serverAddr := server.GetLocalAddr()

	// Create client socket
	client, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Failed to create client socket: %v", err)
	}
	defer client.Close()

	// Test data
	testMessage := []byte("Hello from Linux socket!")

	// Send data from client to server
	go func() {
		time.Sleep(50 * time.Millisecond) // Give server time to start receiving
		n, err := client.SendTo(testMessage, serverAddr.IP, serverAddr.Port)
		if err != nil {
			t.Errorf("Failed to send: %v", err)
			return
		}
		if n != len(testMessage) {
			t.Errorf("Expected to send %d bytes, sent %d", len(testMessage), n)
		}
	}()

	// Receive data on server
	buffer := make([]byte, 1024)
	n, fromAddr, err := server.RecvFrom(buffer)
	if err != nil {
		t.Fatalf("Failed to receive: %v", err)
	}

	receivedMessage := buffer[:n]
	if string(receivedMessage) != string(testMessage) {
		t.Errorf("Message mismatch. Expected: %s, Got: %s", testMessage, receivedMessage)
	}

	if fromAddr.IP == "" || fromAddr.Port == 0 {
		t.Errorf("Expected valid sender address, got IP: %s, Port: %d", fromAddr.IP, fromAddr.Port)
	}
}

func TestLinuxSocketPerformance(t *testing.T) {
	// This test will help us measure our baseline performance
	// Later we'll use this to compare zero-copy improvements
	
	server, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	err = server.Bind("127.0.0.1", 0)
	if err != nil {
		t.Fatalf("Failed to bind server: %v", err)
	}

	client, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	serverAddr := server.GetLocalAddr()
	testData := make([]byte, 1024) // 1KB test packets
	numPackets := 1000

	// Start receiving in background
	go func() {
		buffer := make([]byte, 2048)
		for i := 0; i < numPackets; i++ {
			_, _, err := server.RecvFrom(buffer)
			if err != nil {
				t.Errorf("Failed to receive packet %d: %v", i, err)
				return
			}
		}
	}()

	// Measure send performance
	start := time.Now()
	for i := 0; i < numPackets; i++ {
		_, err := client.SendTo(testData, serverAddr.IP, serverAddr.Port)
		if err != nil {
			t.Fatalf("Failed to send packet %d: %v", i, err)
		}
	}
	duration := time.Since(start)

	// Calculate packets per second
	pps := float64(numPackets) / duration.Seconds()
	
	// This is our baseline - we'll improve this with zero-copy operations
	t.Logf("Baseline performance: %.0f packets/second, %.2f µs/packet", 
		pps, float64(duration.Microseconds())/float64(numPackets))

	// For now, just check we can send at reasonable speed
	if pps < 10000 { // Should be able to do at least 10k pps
		t.Errorf("Performance too low: %.0f pps (expected > 10000)", pps)
	}
}

func TestLinuxSocketNonBlocking(t *testing.T) {
	// Test non-blocking mode - essential for high-performance servers
	socket, err := NewLinuxUDPSocket()
	if err != nil {
		t.Fatalf("Failed to create socket: %v", err)
	}
	defer socket.Close()

	// Set non-blocking mode
	err = socket.SetNonBlocking(true)
	if err != nil {
		t.Fatalf("Failed to set non-blocking: %v", err)
	}

	// Try to receive when no data available - should not block
	buffer := make([]byte, 1024)
	start := time.Now()
	_, _, err = socket.RecvFrom(buffer)
	duration := time.Since(start)

	// Should return immediately with EAGAIN/EWOULDBLOCK error
	if duration > 10*time.Millisecond {
		t.Errorf("Non-blocking recv took too long: %v", duration)
	}

	// Error should indicate no data available (this is expected behavior)
	if err == nil {
		t.Error("Expected error when no data available in non-blocking mode")
	}
}