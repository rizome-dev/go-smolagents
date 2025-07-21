// Copyright 2025 Rizome Labs, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package executors

import (
	"strings"
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
		"timeout":             5 * time.Second,
		"max_memory":          50 * 1024 * 1024,
		"max_output_length":   5000,
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

	// Get the raw result for debugging
	result, execErr := executor.ExecuteWithResult(code)
	if execErr != nil {
		t.Errorf("ExecuteWithResult failed: %v", execErr)
	} else {
		t.Logf("Raw result: %+v", result)
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

func TestGoExecutorFinalAnswer(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()

	t.Run("final_answer detection", func(t *testing.T) {
		code := `final_answer("The answer is 42")`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		if !result.IsFinalAnswer {
			t.Error("Expected IsFinalAnswer to be true")
		}
		if result.FinalAnswer != "The answer is 42" {
			t.Errorf("Expected final answer 'The answer is 42', got %v", result.FinalAnswer)
		}
	})

	t.Run("regular execution without final_answer", func(t *testing.T) {
		code := `result = "regular output"`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		if result.IsFinalAnswer {
			t.Error("Expected IsFinalAnswer to be false")
		}
		if result.Output != "regular output" {
			t.Errorf("Expected output 'regular output', got %v", result.Output)
		}
	})

	t.Run("final_answer with computation", func(t *testing.T) {
		code := `
		sum := 0
		for i := 1; i <= 10; i++ {
			sum += i
		}
		final_answer(fmt.Sprintf("The sum of 1 to 10 is %d", sum))
		`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		if !result.IsFinalAnswer {
			t.Error("Expected IsFinalAnswer to be true")
		}
		expectedAnswer := "The sum of 1 to 10 is 55"
		if result.FinalAnswer != expectedAnswer {
			t.Errorf("Expected final answer '%s', got %v", expectedAnswer, result.FinalAnswer)
		}
	})
}

func TestGoExecutorExecuteWithResult(t *testing.T) {
	// Create executor with strings package authorized
	options := map[string]interface{}{
		"authorized_packages": append(DefaultAuthorizedPackages(), "strings"),
	}
	executor, err := NewGoExecutor(options)
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()

	t.Run("simple computation", func(t *testing.T) {
		code := `result = 10 * 5`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		if result.Output != float64(50) {
			t.Errorf("Expected output 50, got %v", result.Output)
		}
		if result.IsFinalAnswer {
			t.Error("Expected IsFinalAnswer to be false for regular execution")
		}
	})

	t.Run("string manipulation", func(t *testing.T) {
		// Since we're not using the Python-style imports, let's skip this test
		t.Skip("String manipulation requires import handling not yet implemented")
	})

	t.Run("error handling", func(t *testing.T) {
		code := `
		// This should cause an error
		result = 1 / 0
		`
		result, err := executor.ExecuteWithResult(code)
		if err == nil {
			t.Error("Expected error for division by zero")
		}
		if result != nil && result.Stderr == "" {
			t.Error("Expected stderr output for error")
		}
	})

	t.Run("variable state preservation", func(t *testing.T) {
		// Variable persistence between executions is not yet implemented
		t.Skip("Variable state preservation not yet implemented")
	})
}

func TestGoExecutorSyntaxErrors(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()

	testCases := []struct {
		name          string
		code          string
		expectError   bool
		errorContains string
	}{
		{
			name:          "incomplete expression",
			code:          `result = 1 +`,
			expectError:   true,
			errorContains: "syntax error",
		},
		{
			name:          "undefined variable",
			code:          `result = undefined_var`,
			expectError:   true,
			errorContains: "undefined",
		},
		{
			name:          "invalid syntax",
			code:          `if true { result = "missing closing brace"`,
			expectError:   true,
			errorContains: "syntax error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executor.ExecuteWithResult(tc.code)
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tc.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
			_ = result // Suppress unused variable warning
		})
	}
}

func TestGoExecutorSecurityRestrictions(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()

	testCases := []struct {
		name          string
		code          string
		errorContains string
	}{
		{
			name:          "os.Exit usage",
			code:          `os.Exit(0)`,
			errorContains: "unsafe operation",
		},
		{
			name:          "panic usage",
			code:          `panic("test panic")`,
			errorContains: "unsafe operation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executor.ExecuteWithResult(tc.code)
			if err == nil {
				t.Error("Expected security error but got none")
			} else if !strings.Contains(err.Error(), tc.errorContains) {
				t.Errorf("Expected error containing '%s', got: %v", tc.errorContains, err)
			}
		})
	}
}
