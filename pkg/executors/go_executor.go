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

// Package executors provides code execution capabilities for smolagents.
//
// This implements a Go code executor that can safely execute Go code snippets
// while maintaining variable state between executions and providing proper
// error handling and security features.
package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

// ExecutionResult represents the result of code execution
type ExecutionResult struct {
	Output        interface{}            `json:"output"`
	Variables     map[string]interface{} `json:"variables"`
	Stdout        string                 `json:"stdout"`
	Stderr        string                 `json:"stderr"`
	ExitCode      int                    `json:"exit_code"`
	Duration      time.Duration          `json:"duration"`
	IsFinalAnswer bool                   `json:"is_final_answer"`
	FinalAnswer   interface{}            `json:"final_answer,omitempty"`
	Logs          string                 `json:"logs"` // Print outputs captured during execution
}

// GoExecutor implements code execution for Go code snippets
type GoExecutor struct {
	// State management
	variables          map[string]interface{}
	availableTools     map[string]tools.Tool
	authorizedPackages []string
	workingDir         string

	// Configuration
	timeout          time.Duration
	maxMemory        int64
	maxOutputLength  int
	enableNetworking bool
	enableFileSystem bool

	// Execution state
	executionCount int
	mu             sync.RWMutex

	// Code template for execution
	codeTemplate string
}

// NewGoExecutor creates a new Go code executor
func NewGoExecutor(options ...map[string]interface{}) (*GoExecutor, error) {
	executor := &GoExecutor{
		variables:          make(map[string]interface{}),
		availableTools:     make(map[string]tools.Tool),
		authorizedPackages: DefaultAuthorizedPackages(),
		timeout:            30 * time.Second,
		maxMemory:          100 * 1024 * 1024, // 100MB
		maxOutputLength:    10000,
		enableNetworking:   false,
		enableFileSystem:   false,
		executionCount:     0,
		codeTemplate:       defaultGoTemplate,
	}

	// Create working directory
	workDir, err := os.MkdirTemp("", "smolagents-go-executor-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}
	executor.workingDir = workDir

	// Apply options
	if len(options) > 0 {
		opts := options[0]
		if timeout, ok := opts["timeout"].(time.Duration); ok {
			executor.timeout = timeout
		}
		if maxMemory, ok := opts["max_memory"].(int64); ok {
			executor.maxMemory = maxMemory
		}
		if maxOutput, ok := opts["max_output_length"].(int); ok {
			executor.maxOutputLength = maxOutput
		}
		if networking, ok := opts["enable_networking"].(bool); ok {
			executor.enableNetworking = networking
		}
		if filesystem, ok := opts["enable_filesystem"].(bool); ok {
			executor.enableFileSystem = filesystem
		}
		if packages, ok := opts["authorized_packages"].([]string); ok {
			executor.authorizedPackages = packages
		}
	}

	return executor, nil
}

// DefaultAuthorizedPackages returns the default list of authorized Go packages
func DefaultAuthorizedPackages() []string {
	return []string{
		"fmt", "math", "math/rand", "strings", "strconv", "time",
		"encoding/json", "encoding/csv", "encoding/base64",
		"regexp", "sort", "unicode", "unicode/utf8",
		"crypto/md5", "crypto/sha1", "crypto/sha256",
		"path", "path/filepath", "net/url",
		"compress/gzip", "archive/zip",
		"bytes", "bufio", "io", "container/list", "container/heap",
	}
}

// Execute executes Go code and returns the result
func (ge *GoExecutor) Execute(code string, authorizedImports []string) (interface{}, error) {
	result, err := ge.ExecuteRaw(code, authorizedImports)
	if err != nil {
		return nil, err
	}
	return result.Output, nil
}

// ExecuteRaw executes Go code and returns the full execution result
func (ge *GoExecutor) ExecuteRaw(code string, authorizedImports []string) (*ExecutionResult, error) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	ge.executionCount++

	// Validate the code
	if err := ge.validateCode(code, authorizedImports); err != nil {
		return nil, fmt.Errorf("code validation failed: %w", err)
	}

	// Prepare the complete Go program
	program, err := ge.buildProgram(code, authorizedImports)
	if err != nil {
		return nil, fmt.Errorf("failed to build program: %w", err)
	}

	// Execute the program
	result, err := ge.executeProgram(program)
	if err != nil {
		// Return the result even on error for debugging
		if result != nil {
			return result, fmt.Errorf("execution failed: %w", err)
		}
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Parse and store any variable updates
	if err := ge.updateVariables(result); err != nil {
		// Don't fail on variable update errors, just log them
		fmt.Printf("Warning: failed to update variables: %v\n", err)
	}

	return result, nil
}

// validateCode validates the Go code before execution
func (ge *GoExecutor) validateCode(code string, authorizedImports []string) error {
	// Parse the code to check syntax by wrapping it in a function
	fset := token.NewFileSet()
	wrappedCode := fmt.Sprintf(`package main
func main() {
	var result interface{}
	%s
}`, code)
	_, err := parser.ParseFile(fset, "", wrappedCode, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}

	// Check for unauthorized imports
	if err := ge.checkImports(code, authorizedImports); err != nil {
		return fmt.Errorf("unauthorized import: %w", err)
	}

	// Check for unsafe operations
	if err := ge.checkUnsafeOperations(code); err != nil {
		return fmt.Errorf("unsafe operation detected: %w", err)
	}

	return nil
}

// checkImports validates that only authorized imports are used
func (ge *GoExecutor) checkImports(code string, authorizedImports []string) error {
	// Combine authorized packages with explicitly allowed imports
	allowed := make(map[string]bool)
	for _, pkg := range ge.authorizedPackages {
		allowed[pkg] = true
	}
	for _, pkg := range authorizedImports {
		allowed[pkg] = true
	}

	// Parse imports from code
	importRegex := regexp.MustCompile(`import\s+(?:\(\s*([^)]+)\s*\)|"([^"]+)"|([^\s]+))`)
	matches := importRegex.FindAllStringSubmatch(code, -1)

	for _, match := range matches {
		var importPath string
		if match[1] != "" {
			// Multiple imports in parentheses
			lines := strings.Split(match[1], "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					importPath = strings.Trim(line, `"`)
					if !allowed[importPath] {
						return fmt.Errorf("unauthorized import: %s", importPath)
					}
				}
			}
		} else if match[2] != "" {
			// Single import with quotes
			importPath = match[2]
		} else if match[3] != "" {
			// Single import without quotes
			importPath = match[3]
		}

		if importPath != "" && !allowed[importPath] {
			return fmt.Errorf("unauthorized import: %s", importPath)
		}
	}

	return nil
}

// checkUnsafeOperations checks for potentially unsafe operations
func (ge *GoExecutor) checkUnsafeOperations(code string) error {
	unsafePatterns := []string{
		`unsafe\.`,     // unsafe package usage
		`syscall\.`,    // direct syscalls
		`os\.Exit`,     // program termination
		`panic\s*\(`,   // panic calls
		`recover\s*\(`, // recover calls
	}

	if !ge.enableFileSystem {
		unsafePatterns = append(unsafePatterns,
			`os\.Create`, `os\.Open`, `os\.Remove`, `os\.Mkdir`,
			`ioutil\.WriteFile`, `ioutil\.ReadFile`,
			`filepath\.Walk`,
		)
	}

	if !ge.enableNetworking {
		unsafePatterns = append(unsafePatterns,
			`http\.`, `net\.`, `url\.`,
		)
	}

	for _, pattern := range unsafePatterns {
		matched, _ := regexp.MatchString(pattern, code)
		if matched {
			return fmt.Errorf("unsafe operation detected: %s", pattern)
		}
	}

	return nil
}

// extractNewVariables analyzes code to find new variable declarations
func (ge *GoExecutor) extractNewVariables(code string) ([]string, error) {
	// Parse the code
	fset := token.NewFileSet()
	wrappedCode := fmt.Sprintf(`package main
func _() {
%s
}`, code)
	
	node, err := parser.ParseFile(fset, "", wrappedCode, parser.ParseComments)
	if err != nil {
		return nil, nil // Return empty on parse error
	}
	
	var newVars []string
	existingVars := make(map[string]bool)
	for name := range ge.variables {
		existingVars[name] = true
	}
	
	// Find the function body
	var funcBody *ast.BlockStmt
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "_" {
			funcBody = fn.Body
			break
		}
	}
	
	if funcBody == nil {
		return nil, nil
	}
	
	// Track variables found in code
	foundVars := make(map[string]bool)
	
	// Visit all nodes to find variable declarations
	ast.Inspect(funcBody, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			// Handle := and = assignments
			for _, lhs := range x.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					if ident.Name != "_" {
						if !existingVars[ident.Name] && !foundVars[ident.Name] {
							foundVars[ident.Name] = true
							newVars = append(newVars, ident.Name)
						}
					}
				}
			}
		case *ast.DeclStmt:
			// Handle var declarations
			if genDecl, ok := x.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name != "_" {
								if !existingVars[name.Name] && !foundVars[name.Name] {
									foundVars[name.Name] = true
									newVars = append(newVars, name.Name)
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	
	return newVars, nil
}

// transformFunctionVariables handles all function variable declarations,
// converting them from "name := func" to "var name func; name = func" format
func (ge *GoExecutor) transformFunctionVariables(code string) string {
	// More flexible pattern to match function declarations with multiline support
	// This will match: funcName := func(params) returnType {
	funcPattern := regexp.MustCompile(`(?ms)^(\s*)(\w+)\s*:=\s*func\s*\((.*?)\)\s*(.*?)\s*\{`)
	
	// Find all matches with their positions
	allMatches := funcPattern.FindAllStringSubmatchIndex(code, -1)
	
	// Process in reverse order to avoid position shifts
	for i := len(allMatches) - 1; i >= 0; i-- {
		match := funcPattern.FindStringSubmatch(code[allMatches[i][0]:])
		if len(match) < 5 {
			continue
		}
		
		indent := match[1]
		funcName := match[2]
		params := match[3]
		returnType := strings.TrimSpace(match[4])
		
		// Find the matching closing brace for this function
		startPos := allMatches[i][0]
		braceCount := 0
		inString := false
		escapeNext := false
		funcEnd := startPos
		
		for j := startPos; j < len(code); j++ {
			if escapeNext {
				escapeNext = false
				continue
			}
			
			switch code[j] {
			case '\\':
				if inString {
					escapeNext = true
				}
			case '"':
				if !inString {
					inString = true
				} else {
					inString = false
				}
			case '{':
				if !inString {
					braceCount++
				}
			case '}':
				if !inString {
					braceCount--
					if braceCount == 0 {
						funcEnd = j + 1
						break
					}
				}
			}
			
			if funcEnd != startPos {
				break
			}
		}
		
		// Transform ALL function declarations to var declaration format
		// This ensures proper typing and allows recursive calls
		oldDecl := fmt.Sprintf(`%s%s := func(%s) %s {`, indent, funcName, params, returnType)
		// Use backticks to preserve actual newline
		newDecl := fmt.Sprintf(`%svar %s func(%s) %s
%s%s = func(%s) %s {`, 
			indent, funcName, params, returnType,
			indent, funcName, params, returnType)
		
		// Replace in the code
		code = code[:startPos] + strings.Replace(code[startPos:], oldDecl, newDecl, 1)
	}
	
	return code
}

// transformVariableDeclarations transforms := to = for already declared variables
func (ge *GoExecutor) transformVariableDeclarations(code string, declaredVars map[string]bool) string {
	// For now, use a simple regex-based approach
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		// Match variable := value pattern
		if matches := regexp.MustCompile(`^\s*(\w+)\s*:=\s*(.+)$`).FindStringSubmatch(line); len(matches) == 3 {
			varName := matches[1]
			if declaredVars[varName] {
				// Replace := with =
				lines[i] = regexp.MustCompile(`:=`).ReplaceAllString(line, "=")
			}
		}
	}
	return strings.Join(lines, "\n")
}

// generateTypeHelpers creates helper code for type conversions
func (ge *GoExecutor) generateTypeHelpers() string {
	return `// Type conversion helpers
func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0.0
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return __fmt__.Sprint(val)
	}
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	default:
		return false
	}
}`}

// wrapArithmeticOperations wraps variables in type conversion functions for arithmetic
func (ge *GoExecutor) wrapArithmeticOperations(code string, existingVars map[string]interface{}) string {
	// For each existing variable, wrap it in appropriate type conversion
	for varName, value := range existingVars {
		if value == nil {
			continue
		}
		
		// Check if the variable is a function in the code
		funcPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, varName))
		varFuncPattern := regexp.MustCompile(fmt.Sprintf(`\bvar\s+%s\s+func\s*\(`, varName))
		if funcPattern.MatchString(code) || varFuncPattern.MatchString(code) {
			// Skip function variables
			continue
		}
		
		// Determine the appropriate conversion function
		var convFunc string
		switch v := value.(type) {
		case int, int32, int64:
			convFunc = "toInt"
		case float64, float32:
			// Check if it's actually an integer stored as float64 (common with JSON)
			if float64(int(v.(float64))) == v.(float64) {
				convFunc = "toInt"
			} else {
				convFunc = "toFloat64"
			}
		case string:
			convFunc = "toString"
		case bool:
			convFunc = "toBool"
		case func(...interface{}) interface{}:
			// Skip function types
			continue
		default:
			continue
		}
		
		// Replace patterns like "x + 10" with "toInt(x) + 10"
		// Handle different types of assignments
		lines := strings.Split(code, "\n")
		for i, line := range lines {
			// Special handling for for loops
			if strings.Contains(line, "for") && strings.Contains(line, ":=") {
				// Handle for loop conditions that use existing variables
				// Pattern: for i := start; i <= existingVar; i++ 
				forLoopRegex := regexp.MustCompile(`^(\s*for\s+\w+\s*:=\s*[^;]+;\s*)([^;]+)(\s*;\s*[^{]+)(.*)$`)
				if matches := forLoopRegex.FindStringSubmatch(line); len(matches) >= 4 {
					condition := matches[2]
					// Wrap the variable in the condition
					if strings.Contains(condition, varName) {
						wrappedCondition := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, varName)).ReplaceAllString(condition, convFunc+"("+varName+")")
						lines[i] = matches[1] + wrappedCondition + matches[3] + matches[4]
					}
				}
				continue
			}
			
			// Check for compound assignment operators (+=, -=, etc.)
			compoundRegex := regexp.MustCompile(fmt.Sprintf(`^(\s*%s\s*)([+\-*/%%]=)\s*(.+)$`, varName))
			if matches := compoundRegex.FindStringSubmatch(line); len(matches) == 4 {
				// Convert compound assignment to regular assignment
				// e.g., "x += 5" becomes "x = toInt(x) + 5"
				op := matches[2][:1] // Extract the operator (+, -, *, /, %)
				lines[i] = fmt.Sprintf("%s= %s(%s) %s %s", matches[1], convFunc, varName, op, matches[3])
			} else {
				// Check for regular assignment
				assignmentRegex := regexp.MustCompile(fmt.Sprintf(`^(\s*%s\s*=\s*)(.+)$`, varName))
				if matches := assignmentRegex.FindStringSubmatch(line); len(matches) == 3 {
					// Regular assignment, only wrap the RHS
					rhs := matches[2]
					// Replace variable in RHS with wrapped version
					wrappedRhs := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, varName)).ReplaceAllString(rhs, convFunc+"("+varName+")")
					lines[i] = matches[1] + wrappedRhs
				} else if regexp.MustCompile(fmt.Sprintf(`\b%s\s*[+\-*/%%<>=]`, varName)).MatchString(line) ||
				           regexp.MustCompile(fmt.Sprintf(`[+\-*/%%<>=]\s*%s\b`, varName)).MatchString(line) {
					// Not an assignment, wrap all occurrences EXCEPT in Printf/Sprintf calls and function calls
					// Skip if this line contains Printf, Sprintf, Fprintf, etc.
					if !strings.Contains(line, "Printf") && !strings.Contains(line, "Sprintf") && !strings.Contains(line, "Fprintf") {
						// Replace variable occurrences but not function calls
						varPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, varName))
						lines[i] = varPattern.ReplaceAllStringFunc(line, func(match string) string {
							// Check if this match is followed by '(' (function call)
							matchIndex := strings.Index(line, match)
							if matchIndex >= 0 && matchIndex+len(match) < len(line) {
								afterMatch := line[matchIndex+len(match):]
								if strings.TrimSpace(afterMatch) != "" && strings.TrimSpace(afterMatch)[0] == '(' {
									// This is a function call, don't wrap it
									return match
								}
							}
							// Not a function call, wrap it
							return convFunc + "(" + match + ")"
						})
					}
				}
			}
		}
		code = strings.Join(lines, "\n")
	}
	
	return code
}

// detectUsedImports analyzes code to determine which imports are actually used
func (ge *GoExecutor) detectUsedImports(code string, availableImports []string) []string {
	// Map of package names to their import paths
	packageMap := map[string]string{
		"json":      "encoding/json",
		"base64":    "encoding/base64",
		"csv":       "encoding/csv",
		"md5":       "crypto/md5",
		"sha1":      "crypto/sha1",
		"sha256":    "crypto/sha256",
		"gzip":      "compress/gzip",
		"zip":       "archive/zip",
		"url":       "net/url",
		"filepath":  "path/filepath",
		"path":      "path",
		"rand":      "math/rand",
		"math":      "math",
		"strings":   "strings",
		"strconv":   "strconv",
		"time":      "time",
		"regexp":    "regexp",
		"sort":      "sort",
		"unicode":   "unicode",
		"utf8":      "unicode/utf8",
		"bytes":     "bytes",
		"bufio":     "bufio",
		"io":        "io",
		"list":      "container/list",
		"heap":      "container/heap",
		"reflect":   "reflect",
		"os":        "os",
	}
	
	// Always include these core imports
	alwaysInclude := map[string]bool{
		"encoding/json": true, // For result marshaling
		"os":            true, // For os.Exit and error output
		"bytes":         true, // For printBuffer
	}
	
	usedImports := make(map[string]bool)
	
	// Add always-included imports
	for imp := range alwaysInclude {
		usedImports[imp] = true
	}
	
	// Check which packages are used in the code
	for pkg, importPath := range packageMap {
		// Skip if not in available imports
		found := false
		for _, avail := range availableImports {
			if avail == importPath {
				found = true
				break
			}
		}
		if !found && !alwaysInclude[importPath] {
			continue
		}
		
		// Check if package is used
		pattern := fmt.Sprintf(`\b%s\.`, regexp.QuoteMeta(pkg))
		if matched, _ := regexp.MatchString(pattern, code); matched {
			usedImports[importPath] = true
		}
	}
	
	// Convert map to slice
	var result []string
	for imp := range usedImports {
		result = append(result, imp)
	}
	
	// Sort for consistent output
	sort.Strings(result)
	return result
}

// buildProgram creates a complete Go program from the code snippet
func (ge *GoExecutor) buildProgram(code string, authorizedPackages []string) (string, error) {
	// Extract new variables from the code
	newVars, _ := ge.extractNewVariables(code)
	
	// Transform function variables first
	code = ge.transformFunctionVariables(code)
	
	// Transform code to fix variable declarations
	declaredVars := make(map[string]bool)
	for name := range ge.variables {
		declaredVars[name] = true
	}
	for _, name := range newVars {
		declaredVars[name] = true
	}
	transformedCode := ge.transformVariableDeclarations(code, declaredVars)
	
	// Wrap arithmetic operations with all variables in type conversions
	// Include both existing variables and new variables
	allVarsForWrapping := make(map[string]interface{})
	for k, v := range ge.variables {
		allVarsForWrapping[k] = v
	}
	// For new variables, we need to infer their type from initialization
	// Look for patterns like "varName := intLiteral" to set proper default types
	for _, varName := range newVars {
		// Check if the variable is initialized with a function
		funcInitPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, varName))
		if funcInitPattern.MatchString(code) {
			// It's a function, skip it - don't add to wrapping
			continue
		}
		
		// Check if the variable is initialized with an integer literal
		intInitPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*(\d+)\b`, varName))
		if matches := intInitPattern.FindStringSubmatch(code); len(matches) > 1 {
			// It's initialized with an integer literal, use int type
			allVarsForWrapping[varName] = 0
		} else {
			// Default to int for loop variables or other cases
			allVarsForWrapping[varName] = 0
		}
	}
	transformedCode = ge.wrapArithmeticOperations(transformedCode, allVarsForWrapping)
	
	// Combine default and authorized packages
	allAvailablePackages := append(ge.authorizedPackages, authorizedPackages...)
	
	// Detect which imports are actually used
	usedImports := ge.detectUsedImports(transformedCode, allAvailablePackages)
	
	// Build import list
	var imports []string
	for _, pkg := range usedImports {
		if pkg == "fmt" {
			continue // We handle fmt specially
		}
		imports = append(imports, fmt.Sprintf(`"%s"`, pkg))
	}
	// Always add renamed fmt
	imports = append(imports, `__fmt__ "fmt"`)

	// Prepare existing variables as globals - always use interface{} for flexibility
	var variableDeclarations []string
	for name, value := range ge.variables {
		if value == nil {
			variableDeclarations = append(variableDeclarations,
				fmt.Sprintf("var %s interface{}", name))
		} else {
			varValue := ge.formatValue(value)
			variableDeclarations = append(variableDeclarations,
				fmt.Sprintf("var %s interface{} = %s", name, varValue))
		}
	}
	
	// Declare new variables with appropriate types
	funcVarTypes := make(map[string]string)
	for _, varName := range newVars {
		// Check if this is a function variable and extract its signature
		// Check in the ORIGINAL code, not transformedCode, because transformations might have changed it
		// Use (?s) flag to make . match newlines as well
		funcPattern := regexp.MustCompile(fmt.Sprintf(`(?s)\b%s\s*:=\s*func\s*\((.*?)\)\s*(.*?)\s*\{`, regexp.QuoteMeta(varName)))
		varFuncPattern := regexp.MustCompile(fmt.Sprintf(`(?m)\bvar\s+%s\s+func\s*\((.*?)\)\s*(.*?)$`, regexp.QuoteMeta(varName)))
		
		isFuncVar := false
		// Check both original code and transformed code
		if funcPattern.MatchString(code) || funcPattern.MatchString(transformedCode) {
			// It's a function variable, don't declare it as interface{}
			isFuncVar = true
			if matches := funcPattern.FindStringSubmatch(code); len(matches) > 2 {
				params := matches[1]
				returns := strings.TrimSpace(matches[2])
				funcVarTypes[varName] = fmt.Sprintf("func(%s) %s", params, returns)
			} else if matches := funcPattern.FindStringSubmatch(transformedCode); len(matches) > 2 {
				params := matches[1]
				returns := strings.TrimSpace(matches[2])
				funcVarTypes[varName] = fmt.Sprintf("func(%s) %s", params, returns)
			}
		} else if varFuncPattern.MatchString(transformedCode) {
			// It's a var declaration for a function
			isFuncVar = true
			if matches := varFuncPattern.FindStringSubmatch(transformedCode); len(matches) > 2 {
				params := matches[1]
				returns := strings.TrimSpace(matches[2])
				funcVarTypes[varName] = fmt.Sprintf("func(%s) %s", params, returns)
			}
		}
		
		if !isFuncVar {
			// Non-function variables as interface{}
			variableDeclarations = append(variableDeclarations,
				fmt.Sprintf("var %s interface{}", varName))
		}
		// Function variables don't need declaration at top level
	}

	// Build variable capture code for all variables (excluding functions)
	allVars := make([]string, 0, len(ge.variables)+len(newVars))
	for name := range ge.variables {
		allVars = append(allVars, name)
	}
	allVars = append(allVars, newVars...)
	
	// Use funcVarTypes to identify function variables to exclude from capture
	funcVars := make(map[string]bool)
	for _, varName := range allVars {
		if _, isFunc := funcVarTypes[varName]; isFunc {
			funcVars[varName] = true
		} else {
			// Also check existing variables that might be functions
			funcPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, varName))
			varFuncPattern := regexp.MustCompile(fmt.Sprintf(`\bvar\s+%s\s+func\s*\(`, varName))
			// Also check if it's been transformed to var declaration
			if funcPattern.MatchString(transformedCode) || varFuncPattern.MatchString(transformedCode) {
				funcVars[varName] = true
			}
		}
	}
	
	var variableCaptures []string
	for _, name := range allVars {
		// Skip function variables as they can't be serialized
		if !funcVars[name] {
			variableCaptures = append(variableCaptures,
				fmt.Sprintf(`"%s": %s`, name, name))
		}
	}
	
	// Handle empty variable captures to avoid syntax errors  
	variableCapturesStr := ""
	if len(variableCaptures) > 0 {
		// For single item, keep it on same line; for multiple, use newlines
		if len(variableCaptures) == 1 {
			variableCapturesStr = variableCaptures[0]
		} else {
			variableCapturesStr = "\n\t\t" + strings.Join(variableCaptures, ",\n\t\t") + ",\n\t"
		}
	} else {
		variableCapturesStr = ""
	}

	// Build the complete program
	program := fmt.Sprintf(`package main

import (
	%s
)

%s

// Global print output buffer
var printBuffer bytes.Buffer

// Custom fmt wrapper to capture print output
type customFmt struct{}

var fmt = customFmt{}

%s

func (customFmt) Print(a ...interface{}) (n int, err error) {
	return __fmt__.Fprint(&printBuffer, a...)
}

func (customFmt) Printf(format string, a ...interface{}) (n int, err error) {
	return __fmt__.Fprintf(&printBuffer, format, a...)
}

func (customFmt) Println(a ...interface{}) (n int, err error) {
	return __fmt__.Fprintln(&printBuffer, a...)
}

func (customFmt) Sprint(a ...interface{}) string {
	return __fmt__.Sprint(a...)
}

func (customFmt) Sprintf(format string, a ...interface{}) string {
	return __fmt__.Sprintf(format, a...)
}

func (customFmt) Sprintln(a ...interface{}) string {
	return __fmt__.Sprintln(a...)
}

// final_answer provides the final answer to the task
func final_answer(answer interface{}) {
	output := map[string]interface{}{
		"is_final_answer": true,
		"final_answer": answer,
		"variables": getCurrentVariables(),
		"logs": printBuffer.String(),
	}
	
	jsonOutput, err := json.Marshal(output)
	if err != nil {
		__fmt__.Fprintf(os.Stderr, "Failed to marshal final answer: %%v\n", err)
		os.Exit(1)
	}
	
	__fmt__.Print(string(jsonOutput))
	os.Exit(0)
}

func getCurrentVariables() map[string]interface{} {
	return map[string]interface{}{%s}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			__fmt__.Fprintf(os.Stderr, "Panic: %%v\n", r)
			os.Exit(1)
		}
	}()
	
	// User code starts here
	%s
	// User code ends here
	
	// Capture variables state
	variables := getCurrentVariables()
	
	// Determine the result value
	var outputResult interface{}
	if val, exists := variables["result"]; exists {
		outputResult = val
	}
	
	// Output result
	output := map[string]interface{}{
		"result": outputResult,
		"variables": variables,
		"is_final_answer": false,
		"logs": printBuffer.String(),
	}
	
	jsonOutput, err := json.Marshal(output)
	if err != nil {
		__fmt__.Fprintf(os.Stderr, "Failed to marshal output: %%v\n", err)
		os.Exit(1)
	}
	
	__fmt__.Print(string(jsonOutput))
}`,
		strings.Join(imports, "\n\t"),
		strings.Join(variableDeclarations, "\n"),
		ge.generateTypeHelpers(),
		variableCapturesStr,
		transformedCode,
	)

	return program, nil
}

// formatValue formats a Go value for code generation
func (ge *GoExecutor) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf(`"%s"`, strings.ReplaceAll(v, `"`, `\"`))
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return "nil"
	default:
		// For complex types, use JSON marshaling
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "nil"
		}
		return fmt.Sprintf(`json.RawMessage(%q)`, string(jsonBytes))
	}
}

// executeProgram executes the Go program and returns the result
func (ge *GoExecutor) executeProgram(program string) (*ExecutionResult, error) {
	// Write program to file
	filename := fmt.Sprintf("program_%d.go", ge.executionCount)
	filePath := filepath.Join(ge.workingDir, filename)

	if err := os.WriteFile(filePath, []byte(program), 0644); err != nil {
		return nil, fmt.Errorf("failed to write program file: %w", err)
	}

	// Set up execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ge.timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(ctx, "go", "run", filePath)
	cmd.Dir = ge.workingDir

	// Set up environment restrictions
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"GOPATH=" + ge.workingDir,
		"GOCACHE=" + filepath.Join(ge.workingDir, ".cache"),
		fmt.Sprintf("GOMAXPROCS=%d", 1), // Limit CPU usage
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("execution timeout after %v", ge.timeout)
		}
		result.ExitCode = 1
		errorMsg := fmt.Sprintf("execution failed: %v", err)
		if stderr.Len() > 0 {
			errorMsg += fmt.Sprintf(", stderr: %s", stderr.String())
		}
		return result, fmt.Errorf("%s", errorMsg)
	}

	// Parse output
	if stdout.Len() > 0 {
		var output map[string]interface{}
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			// If JSON parsing fails, assume it's print output
			result.Logs = stdout.String()
			result.Output = nil
		} else {
			// Check if it's a final answer
			if isFinal, ok := output["is_final_answer"].(bool); ok && isFinal {
				result.IsFinalAnswer = true
				result.FinalAnswer = output["final_answer"]
				result.Output = output["final_answer"]
			} else {
				result.Output = output["result"]
			}

			if vars, ok := output["variables"].(map[string]interface{}); ok {
				result.Variables = vars
			}
			
			// Extract print logs if present
			if logs, ok := output["logs"].(string); ok {
				result.Logs = logs
			}
		}
	}

	return result, nil
}

// updateVariables updates the executor's variable state from execution result
func (ge *GoExecutor) updateVariables(result *ExecutionResult) error {
	if result.Variables == nil {
		return nil
	}

	for name, value := range result.Variables {
		ge.variables[name] = value
	}

	return nil
}

// SendVariables updates the executor's variable state
func (ge *GoExecutor) SendVariables(variables map[string]interface{}) error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	for name, value := range variables {
		ge.variables[name] = value
	}

	return nil
}

// SendTools makes tools available to the executor
func (ge *GoExecutor) SendTools(tools map[string]tools.Tool) error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	for name, tool := range tools {
		ge.availableTools[name] = tool
	}

	return nil
}

// GetState returns the current variable state
func (ge *GoExecutor) GetState() map[string]interface{} {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	state := make(map[string]interface{})
	for name, value := range ge.variables {
		state[name] = value
	}

	return state
}

// Reset clears all variables and state
func (ge *GoExecutor) Reset() error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	ge.variables = make(map[string]interface{})
	ge.executionCount = 0

	// Clean up working directory
	if err := os.RemoveAll(ge.workingDir); err != nil {
		return fmt.Errorf("failed to clean working directory: %w", err)
	}

	// Create new working directory
	workDir, err := os.MkdirTemp("", "smolagents-go-executor-*")
	if err != nil {
		return fmt.Errorf("failed to create new working directory: %w", err)
	}
	ge.workingDir = workDir

	return nil
}

// SetTimeout sets the execution timeout
func (ge *GoExecutor) SetTimeout(timeout time.Duration) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.timeout = timeout
}

// SetMaxMemory sets the maximum memory usage
func (ge *GoExecutor) SetMaxMemory(maxMemory int64) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.maxMemory = maxMemory
}

// SetAuthorizedPackages sets the list of authorized Go packages
func (ge *GoExecutor) SetAuthorizedPackages(packages []string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.authorizedPackages = packages
}

// ExecuteWithResult executes code and returns a structured result with final answer detection
func (ge *GoExecutor) ExecuteWithResult(code string) (*ExecutionResult, error) {
	// Use default authorized imports
	return ge.ExecuteRaw(code, []string{})
}

// Close cleans up the executor
func (ge *GoExecutor) Close() error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	return os.RemoveAll(ge.workingDir)
}

// defaultGoTemplate is the default template for Go code execution
const defaultGoTemplate = `package main

import (
	{{IMPORTS}}
)

{{VARIABLES}}

func main() {
	{{CODE}}
}`