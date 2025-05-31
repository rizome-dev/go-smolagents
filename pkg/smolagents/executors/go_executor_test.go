package executors

import (
	"testing"
	"time"
)

func TestNewGoExecutor(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	
	if executor == nil {
		t.Fatal("Expected non-nil executor")
	}
	
	if len(executor.authorizedPackages) == 0 {
		t.Error("Expected default authorized packages")
	}
	
	// Clean up
	executor.Close()
}

func TestGoExecutorWithOptions(t *testing.T) {
	options := map[string]interface{}{
		"timeout":           5 * time.Second,
		"max_memory":        50 * 1024 * 1024,
		"max_output_length": 5000,
		"authorized_packages": []string{"fmt", "math"},
	}
	
	executor, err := NewGoExecutor(options)
	if err != nil {
		t.Fatalf("Failed to create GoExecutor with options: %v", err)
	}
	
	if executor.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", executor.timeout)
	}
	
	if len(executor.authorizedPackages) != 2 {
		t.Errorf("Expected 2 authorized packages, got %d", len(executor.authorizedPackages))
	}
	
	executor.Close()
}

func TestGoExecutorSimpleCode(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()
	
	code := `
	result = 2 + 3
	fmt.Printf("Result: %d\n", result)
	`
	
	output, err := executor.Execute(code, []string{"fmt"})
	if err != nil {
		t.Errorf("Failed to execute simple code: %v", err)
	}
	
	if output == nil {
		t.Error("Expected non-nil output")
	}
}

func TestGoExecutorVariablePersistence(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()
	
	// Set initial variables
	variables := map[string]interface{}{
		"x": 10,
		"y": 20,
	}
	
	err = executor.SendVariables(variables)
	if err != nil {
		t.Errorf("Failed to send variables: %v", err)
	}
	
	state := executor.GetState()
	if state["x"] != 10 || state["y"] != 20 {
		t.Error("Variables not properly set")
	}
}

func TestGoExecutorReset(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()
	
	// Set some variables
	variables := map[string]interface{}{
		"test": "value",
	}
	executor.SendVariables(variables)
	
	// Reset
	err = executor.Reset()
	if err != nil {
		t.Errorf("Failed to reset executor: %v", err)
	}
	
	state := executor.GetState()
	if len(state) != 0 {
		t.Error("Expected empty state after reset")
	}
}

func TestGoExecutorCodeValidation(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()
	
	// Test unauthorized import
	unauthorizedCode := `
	import "os"
	os.Exit(1)
	`
	
	_, err = executor.Execute(unauthorizedCode, []string{"fmt"})
	if err == nil {
		t.Error("Expected error for unauthorized import")
	}
}

func TestGoExecutorTimeout(t *testing.T) {
	options := map[string]interface{}{
		"timeout": 100 * time.Millisecond,
	}
	
	executor, err := NewGoExecutor(options)
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()
	
	// Code that should timeout
	longRunningCode := `
	for i := 0; i < 1000000000; i++ {
		// Long running loop
	}
	`
	
	_, err = executor.Execute(longRunningCode, []string{})
	if err == nil {
		t.Error("Expected timeout error")
	}
}