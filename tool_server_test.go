package ai_test

import (
	"sync"
	"testing"

	"github.com/liuzl/ai"
)

// TestToolServerManagerConcurrency tests that ToolServerManager is safe for concurrent use.
func TestToolServerManagerConcurrency(t *testing.T) {
	manager := ai.NewToolServerManager()

	// Number of concurrent operations
	numGoroutines := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 operations per goroutine

	// Concurrent AddRemoteServer operations
	for i := range numGoroutines {
		go func(idx int) {
			defer wg.Done()
			serverName := "test-server-" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_ = manager.AddRemoteServer(serverName, "http://localhost:8080")
		}(i)
	}

	// Concurrent ListServerNames operations
	for range numGoroutines {
		go func() {
			defer wg.Done()
			_ = manager.ListServerNames()
		}()
	}

	// Concurrent GetClient operations
	for i := range numGoroutines {
		go func(idx int) {
			defer wg.Done()
			serverName := "test-server-" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_, _ = manager.GetClient(serverName)
		}(i)
	}

	wg.Wait()

	// Verify we can still access the manager after concurrent operations
	names := manager.ListServerNames()
	if len(names) == 0 {
		t.Error("Expected at least some servers to be registered")
	}
}

// TestToolServerManagerAddDuplicate tests that adding a duplicate server returns an error.
func TestToolServerManagerAddDuplicate(t *testing.T) {
	manager := ai.NewToolServerManager()

	err := manager.AddRemoteServer("test-server", "http://localhost:8080")
	if err != nil {
		t.Fatalf("First AddRemoteServer failed: %v", err)
	}

	// Try to add the same server again
	err = manager.AddRemoteServer("test-server", "http://localhost:8080")
	if err == nil {
		t.Error("Expected error when adding duplicate server, got nil")
	}
}

// TestToolServerManagerEmptyURL tests that empty URL returns an error.
func TestToolServerManagerEmptyURL(t *testing.T) {
	manager := ai.NewToolServerManager()

	err := manager.AddRemoteServer("test-server", "")
	if err == nil {
		t.Error("Expected error for empty URL, got nil")
	}
}

// TestToolServerManagerGetNonExistent tests getting a non-existent server.
func TestToolServerManagerGetNonExistent(t *testing.T) {
	manager := ai.NewToolServerManager()

	client, exists := manager.GetClient("non-existent")
	if exists {
		t.Error("Expected exists to be false for non-existent server")
	}
	if client != nil {
		t.Error("Expected nil client for non-existent server")
	}
}

// TestToolServerManagerListEmpty tests listing servers in an empty manager.
func TestToolServerManagerListEmpty(t *testing.T) {
	manager := ai.NewToolServerManager()

	names := manager.ListServerNames()
	if len(names) != 0 {
		t.Errorf("Expected 0 servers in empty manager, got %d", len(names))
	}
}

// TestToolServerManagerListMultiple tests listing multiple servers.
func TestToolServerManagerListMultiple(t *testing.T) {
	manager := ai.NewToolServerManager()

	servers := []string{"server1", "server2", "server3"}
	for _, name := range servers {
		err := manager.AddRemoteServer(name, "http://localhost:8080")
		if err != nil {
			t.Fatalf("Failed to add server %s: %v", name, err)
		}
	}

	names := manager.ListServerNames()
	if len(names) != len(servers) {
		t.Errorf("Expected %d servers, got %d", len(servers), len(names))
	}

	// Verify all server names are present
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	for _, expected := range servers {
		if !nameSet[expected] {
			t.Errorf("Expected server %s not found in list", expected)
		}
	}
}

// TestToolServerManagerGetExisting tests getting an existing server.
func TestToolServerManagerGetExisting(t *testing.T) {
	manager := ai.NewToolServerManager()

	err := manager.AddRemoteServer("test-server", "http://localhost:8080")
	if err != nil {
		t.Fatalf("AddRemoteServer failed: %v", err)
	}

	client, exists := manager.GetClient("test-server")
	if !exists {
		t.Error("Expected exists to be true for existing server")
	}
	if client == nil {
		t.Error("Expected non-nil client for existing server")
	}
}
