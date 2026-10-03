package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSingleInstanceSocketCommunication(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "clipboard-gnome-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sockPath := filepath.Join(tempDir, "test.sock")
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("Failed to listen on unix socket: %v", err)
	}
	defer listener.Close()

	mgr := &SingleInstanceManager{
		sockPath:  sockPath,
		listener:  listener,
		startTime: time.Now().Unix(),
	}
	go mgr.serveSocket()

	// Connect as secondary instance
	conn, err := net.DialTimeout("unix", sockPath, time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to test socket: %v", err)
	}
	defer conn.Close()

	// Send status request
	req := InstanceRequest{Action: "status"}
	reqData, _ := json.Marshal(req)
	_, err = conn.Write(append(reqData, '\n'))
	if err != nil {
		t.Fatalf("Failed to write to socket: %v", err)
	}

	reader := bufio.NewReader(conn)
	respLine, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var resp InstanceResponse
	if err := json.Unmarshal(respLine, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != Version {
		t.Errorf("Expected version %q, got %q", Version, resp.Version)
	}
	if resp.PID != os.Getpid() {
		t.Errorf("Expected PID %d, got %d", os.Getpid(), resp.PID)
	}

	// Send subsequent show request on the same connection
	showReq := InstanceRequest{Action: "show", Version: Version, PID: os.Getpid()}
	showData, _ := json.Marshal(showReq)
	_, err = conn.Write(append(showData, '\n'))
	if err != nil {
		t.Fatalf("Failed to write show request on same connection: %v", err)
	}

	showRespLine, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("Failed to read show response: %v", err)
	}

	var showResp InstanceResponse
	if err := json.Unmarshal(showRespLine, &showResp); err != nil {
		t.Fatalf("Failed to unmarshal show response: %v", err)
	}
	if showResp.Status != "ok" {
		t.Errorf("Expected show status 'ok', got %q", showResp.Status)
	}
}
