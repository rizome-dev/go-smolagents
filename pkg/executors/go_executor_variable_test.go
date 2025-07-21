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
	"testing"
)

func TestGoExecutorVariablePersistenceAcrossExecutions(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create GoExecutor: %v", err)
	}
	defer executor.Close()

	t.Run("declare and use variable across executions", func(t *testing.T) {
		// First execution: declare variable
		code1 := `x := 5
fmt.Println("x =", x)`
		result1, err := executor.ExecuteWithResult(code1)
		if err != nil {
			t.Errorf("First execution failed: %v", err)
			if result1 != nil {
				t.Logf("Stderr: %s", result1.Stderr)
			}
		}

		// Check the variable was captured
		if result1.Variables["x"] != float64(5) {
			t.Errorf("Expected x = 5, got %v", result1.Variables["x"])
		}

		// Second execution: use the variable
		code2 := `x = x + 10
fmt.Println("x =", x)`
		result2, err := executor.ExecuteWithResult(code2)
		if err != nil {
			t.Errorf("Second execution failed: %v", err)
			if result2 != nil {
				t.Logf("Stderr: %s", result2.Stderr)
			}
		}

		// Check the variable was updated
		if result2.Variables["x"] != float64(15) {
			t.Errorf("Expected x = 15, got %v", result2.Variables["x"])
		}

		// Third execution: use result variable
		code3 := `result = x * 2`
		result3, err := executor.ExecuteWithResult(code3)
		if err != nil {
			t.Errorf("Third execution failed: %v", err)
		}

		if result3.Output != float64(30) {
			t.Errorf("Expected result = 30, got %v", result3.Output)
		}
	})

	t.Run("multiple variables", func(t *testing.T) {
		executor.Reset() // Start fresh

		// Declare multiple variables
		code1 := `
a := 10
b := 20
c := "hello"
`
		_, err := executor.ExecuteWithResult(code1)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}

		// Use all variables
		code2 := `
d := a + b
e := c + " world"
result = d
`
		result2, err := executor.ExecuteWithResult(code2)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}

		if result2.Output != float64(30) {
			t.Errorf("Expected result = 30, got %v", result2.Output)
		}

		if result2.Variables["e"] != "hello world" {
			t.Errorf("Expected e = 'hello world', got %v", result2.Variables["e"])
		}
	})

	t.Run("variable type changes", func(t *testing.T) {
		executor.Reset() // Start fresh

		// Start with int
		code1 := `x := 42`
		_, err := executor.ExecuteWithResult(code1)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}

		// Change to string
		code2 := `x = "now a string"`
		result2, err := executor.ExecuteWithResult(code2)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}

		if result2.Variables["x"] != "now a string" {
			t.Errorf("Expected x = 'now a string', got %v", result2.Variables["x"])
		}
	})
}
