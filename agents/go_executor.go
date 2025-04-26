// Package agents provides agent implementations for various types of AI agents
package agents

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// GoExecutor is an interface for executing Go code
type GoExecutor interface {
	// Execute runs the provided Go code and returns the output
	Execute(code string) (string, error)

	// SendVariables sends variables to the Go environment
	SendVariables(variables map[string]interface{}) error

	// SendTools makes tools available to the Go environment
	SendTools(tools map[string]interface{}) error
}

// GoSandboxExecutor implements GoExecutor using a temporary file approach
type GoSandboxExecutor struct {
	mutex      sync.Mutex
	tempDir    string
	variables  map[string]interface{}
	tools      map[string]interface{}
	moduleName string
}

// NewGoSandboxExecutor creates a new GoSandboxExecutor
func NewGoSandboxExecutor() (*GoSandboxExecutor, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "go-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Initialize Go module
	moduleName := "sandbox"
	cmd := exec.Command("go", "mod", "init", moduleName)
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to initialize Go module: %w", err)
	}

	return &GoSandboxExecutor{
		tempDir:    tempDir,
		variables:  make(map[string]interface{}),
		tools:      make(map[string]interface{}),
		moduleName: moduleName,
	}, nil
}

// Execute runs the provided Go code and returns the output
func (e *GoSandboxExecutor) Execute(code string) (string, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Create temporary Go file
	mainFile := filepath.Join(e.tempDir, "main.go")

	// Parse the code to see if it contains a main function
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "package main\n"+code, 0)
	if err != nil {
		// If parsing fails, wrap the code in a main function
		wrappedCode := fmt.Sprintf(`package main

import (
	"fmt"
)

// Variables from agent state
%s

// Tools from agent
%s

func main() {
	%s
}
`, e.generateVariableDeclarations(), e.generateToolDeclarations(), code)

		if err := os.WriteFile(mainFile, []byte(wrappedCode), 0644); err != nil {
			return "", fmt.Errorf("failed to write Go file: %w", err)
		}
	} else {
		// Check if the code already contains a main function
		hasMain := false
		ast.Inspect(f, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok {
				if funcDecl.Name.Name == "main" {
					hasMain = true
					return false
				}
			}
			return true
		})

		if hasMain {
			// If it already has a main function, add the imports and variables
			modifiedCode := fmt.Sprintf(`package main

import (
	"fmt"
)

// Variables from agent state
%s

// Tools from agent
%s

%s`, e.generateVariableDeclarations(), e.generateToolDeclarations(), code)

			if err := os.WriteFile(mainFile, []byte(modifiedCode), 0644); err != nil {
				return "", fmt.Errorf("failed to write Go file: %w", err)
			}
		} else {
			// Wrap the code in a main function
			wrappedCode := fmt.Sprintf(`package main

import (
	"fmt"
)

// Variables from agent state
%s

// Tools from agent
%s

func main() {
	%s
}
`, e.generateVariableDeclarations(), e.generateToolDeclarations(), code)

			if err := os.WriteFile(mainFile, []byte(wrappedCode), 0644); err != nil {
				return "", fmt.Errorf("failed to write Go file: %w", err)
			}
		}
	}

	// Build and run the Go code
	cmd := exec.Command("go", "run", mainFile)
	cmd.Dir = e.tempDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Return both stdout and stderr if there's an error
		if stdout.Len() > 0 {
			return fmt.Sprintf("Output:\n%s\nError:\n%s", stdout.String(), stderr.String()), err
		}
		return stderr.String(), err
	}

	return stdout.String(), nil
}

// SendVariables sends variables to the Go environment
func (e *GoSandboxExecutor) SendVariables(variables map[string]interface{}) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for k, v := range variables {
		e.variables[k] = v
	}
	return nil
}

// SendTools makes tools available to the Go environment
func (e *GoSandboxExecutor) SendTools(tools map[string]interface{}) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for k, v := range tools {
		e.tools[k] = v
	}
	return nil
}

// generateVariableDeclarations generates Go code for variable declarations
func (e *GoSandboxExecutor) generateVariableDeclarations() string {
	var result bytes.Buffer
	for k, v := range e.variables {
		// Generate a variable declaration based on the type
		switch v := v.(type) {
		case string:
			fmt.Fprintf(&result, "var %s string = \"%s\"\n", k, v)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			fmt.Fprintf(&result, "var %s int = %v\n", k, v)
		case float32, float64:
			fmt.Fprintf(&result, "var %s float64 = %v\n", k, v)
		case bool:
			fmt.Fprintf(&result, "var %s bool = %v\n", k, v)
		case []interface{}:
			fmt.Fprintf(&result, "var %s = []interface{}{", k)
			for i, item := range v {
				if i > 0 {
					result.WriteString(", ")
				}
				switch item := item.(type) {
				case string:
					fmt.Fprintf(&result, "\"%s\"", item)
				default:
					fmt.Fprintf(&result, "%v", item)
				}
			}
			result.WriteString("}\n")
		case map[string]interface{}:
			fmt.Fprintf(&result, "var %s = map[string]interface{}{\n", k)
			for mk, mv := range v {
				switch mv := mv.(type) {
				case string:
					fmt.Fprintf(&result, "\t\"%s\": \"%s\",\n", mk, mv)
				default:
					fmt.Fprintf(&result, "\t\"%s\": %v,\n", mk, mv)
				}
			}
			result.WriteString("}\n")
		default:
			fmt.Fprintf(&result, "// Variable %s has unsupported type %T\n", k, v)
		}
	}
	return result.String()
}

// generateToolDeclarations generates Go code for tool declarations
func (e *GoSandboxExecutor) generateToolDeclarations() string {
	var result bytes.Buffer
	for k, v := range e.tools {
		fmt.Fprintf(&result, "// Tool %s is available as %T\n", k, v)

		// Create a wrapper function for each tool
		fmt.Fprintf(&result, "func %s(args ...interface{}) interface{} {\n", k)
		fmt.Fprintf(&result, "\tfmt.Printf(\"Tool %s called with args: %%v\\n\", args)\n", k)
		fmt.Fprintf(&result, "\treturn nil // Tool execution is handled by the agent\n")
		result.WriteString("}\n\n")
	}
	return result.String()
}

// Cleanup removes the temporary directory
func (e *GoSandboxExecutor) Cleanup() error {
	return os.RemoveAll(e.tempDir)
}
