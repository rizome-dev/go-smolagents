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

func TestGoExecutorRecursiveFunctions(t *testing.T) {
	executor, err := NewGoExecutor()
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Close()

	t.Run("recursive factorial function", func(t *testing.T) {
		code := `
factorial := func(n int) int {
    if n <= 1 {
        return 1
    }
    return n * factorial(n-1)
}

result := factorial(5)
final_answer(result)
`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if !result.IsFinalAnswer {
			t.Error("Expected final answer")
		}

		// Type assert since FinalAnswer is interface{}
		// JSON unmarshaling often returns numbers as float64
		switch v := result.FinalAnswer.(type) {
		case int:
			if v != 120 {
				t.Errorf("Expected factorial(5) = 120, got %v", v)
			}
		case float64:
			if int(v) != 120 {
				t.Errorf("Expected factorial(5) = 120, got %v", int(v))
			}
		default:
			t.Errorf("Expected factorial(5) = 120, got %v (type: %T)", result.FinalAnswer, result.FinalAnswer)
		}
	})

	t.Run("recursive fibonacci function", func(t *testing.T) {
		code := `
fib := func(n int) int {
    if n <= 1 {
        return n
    }
    return fib(n-1) + fib(n-2)
}

result := fib(10)
final_answer(result)
`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if !result.IsFinalAnswer {
			t.Error("Expected final answer")
		}

		// Type assert since FinalAnswer is interface{}
		// JSON unmarshaling often returns numbers as float64
		switch v := result.FinalAnswer.(type) {
		case int:
			if v != 55 {
				t.Errorf("Expected fib(10) = 55, got %v", v)
			}
		case float64:
			if int(v) != 55 {
				t.Errorf("Expected fib(10) = 55, got %v", int(v))
			}
		default:
			t.Errorf("Expected fib(10) = 55, got %v (type: %T)", result.FinalAnswer, result.FinalAnswer)
		}
	})

	t.Run("non-recursive function should work unchanged", func(t *testing.T) {
		code := `
double := func(n int) int {
    return n * 2
}

result := double(21)
final_answer(result)
`
		result, err := executor.ExecuteWithResult(code)
		if err != nil {
			t.Fatalf("Execution failed: %v", err)
		}

		if !result.IsFinalAnswer {
			t.Error("Expected final answer")
		}

		// Type assert since FinalAnswer is interface{}
		// JSON unmarshaling often returns numbers as float64
		switch v := result.FinalAnswer.(type) {
		case int:
			if v != 42 {
				t.Errorf("Expected double(21) = 42, got %v", v)
			}
		case float64:
			if int(v) != 42 {
				t.Errorf("Expected double(21) = 42, got %v", int(v))
			}
		default:
			t.Errorf("Expected double(21) = 42, got %v (type: %T)", result.FinalAnswer, result.FinalAnswer)
		}
	})
}
