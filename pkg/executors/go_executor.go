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
// This includes the entire Go standard library for maximum flexibility
func DefaultAuthorizedPackages() []string {
	return GetGoStandardLibraryPackages()
}

// GetGoStandardLibraryPackages returns a comprehensive list of Go standard library packages
func GetGoStandardLibraryPackages() []string {
	return []string{
		// Archive formats
		"archive/tar", "archive/zip",
		
		// Buffers and I/O
		"bufio", "bytes", "io", "io/fs", "io/ioutil",
		
		// Compression
		"compress/bzip2", "compress/flate", "compress/gzip", "compress/lzw", "compress/zlib",
		
		// Containers
		"container/heap", "container/list", "container/ring",
		
		// Context
		"context",
		
		// Cryptography
		"crypto", "crypto/aes", "crypto/cipher", "crypto/des", "crypto/dsa",
		"crypto/ecdh", "crypto/ecdsa", "crypto/ed25519", "crypto/elliptic",
		"crypto/hmac", "crypto/md5", "crypto/rand", "crypto/rc4", "crypto/rsa",
		"crypto/sha1", "crypto/sha256", "crypto/sha512", "crypto/subtle",
		"crypto/tls", "crypto/x509", "crypto/x509/pkix",
		
		// Database
		"database/sql", "database/sql/driver",
		
		// Debug
		"debug/buildinfo", "debug/dwarf", "debug/elf", "debug/gosym",
		"debug/macho", "debug/pe", "debug/plan9obj",
		
		// Embed
		"embed",
		
		// Encoding
		"encoding", "encoding/ascii85", "encoding/asn1", "encoding/base32",
		"encoding/base64", "encoding/binary", "encoding/csv", "encoding/gob",
		"encoding/hex", "encoding/json", "encoding/pem", "encoding/xml",
		
		// Errors
		"errors",
		
		// Expvar
		"expvar",
		
		// Flag
		"flag",
		
		// Fmt
		"fmt",
		
		// Go language
		"go/ast", "go/build", "go/build/constraint", "go/constant", "go/doc",
		"go/doc/comment", "go/format", "go/importer", "go/parser", "go/printer",
		"go/scanner", "go/token", "go/types",
		
		// Hash
		"hash", "hash/adler32", "hash/crc32", "hash/crc64", "hash/fnv",
		"hash/maphash",
		
		// HTML
		"html", "html/template",
		
		// Image
		"image", "image/color", "image/color/palette", "image/draw",
		"image/gif", "image/jpeg", "image/png",
		
		// Index
		"index/suffixarray",
		
		// Log
		"log", "log/slog", "log/syslog",
		
		// Maps
		"maps",
		
		// Math
		"math", "math/big", "math/bits", "math/cmplx", "math/rand", "math/rand/v2",
		
		// MIME
		"mime", "mime/multipart", "mime/quotedprintable",
		
		// Net
		"net", "net/http", "net/http/cgi", "net/http/cookiejar", "net/http/fcgi",
		"net/http/httptest", "net/http/httptrace", "net/http/httputil",
		"net/http/internal", "net/http/pprof", "net/mail", "net/netip",
		"net/rpc", "net/rpc/jsonrpc", "net/smtp", "net/textproto", "net/url",
		
		// OS
		"os", "os/exec", "os/signal", "os/user",
		
		// Path
		"path", "path/filepath",
		
		// Plugin
		"plugin",
		
		// Reflect
		"reflect",
		
		// Regexp
		"regexp", "regexp/syntax",
		
		// Runtime
		"runtime", "runtime/cgo", "runtime/coverage", "runtime/debug",
		"runtime/metrics", "runtime/pprof", "runtime/race", "runtime/trace",
		
		// Slices
		"slices",
		
		// Sort
		"sort",
		
		// Strconv
		"strconv",
		
		// Strings
		"strings",
		
		// Sync
		"sync", "sync/atomic",
		
		// Syscall
		"syscall",
		
		// Testing
		"testing", "testing/fstest", "testing/iotest", "testing/quick",
		"testing/slogtest",
		
		// Text
		"text/scanner", "text/tabwriter", "text/template", "text/template/parse",
		
		// Time
		"time", "time/tzdata",
		
		// Unicode
		"unicode", "unicode/utf16", "unicode/utf8",
		
		// Unsafe
		"unsafe",
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

	// Extract imports from the code and clean it
	extractedImports, cleanedCode := ge.extractAndRemoveImports(code)
	
	// Merge extracted imports with authorized imports
	allAuthorizedImports := append(authorizedImports, extractedImports...)

	// Validate the cleaned code
	if err := ge.validateCode(cleanedCode, allAuthorizedImports); err != nil {
		return nil, fmt.Errorf("code validation failed: %w", err)
	}

	// Prepare the complete Go program using cleaned code
	program, err := ge.buildProgram(cleanedCode, allAuthorizedImports)
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
	// Transform function definitions to variable assignments first
	transformedCode := ge.transformStandaloneFunctions(code)
	
	// Parse the code to check syntax by wrapping it in a function
	fset := token.NewFileSet()
	
	testWrapped := fmt.Sprintf(`package main
func _() {
	%s
}`, transformedCode)
	
	_, err := parser.ParseFile(fset, "", testWrapped, parser.ParseComments)
	if err != nil {
		// Return a more helpful error message
		return fmt.Errorf("syntax error: %w", err)
	}

	// Check for unauthorized imports (should be none after extraction)
	if err := ge.checkImports(code, authorizedImports); err != nil {
		return fmt.Errorf("unauthorized import: %w", err)
	}

	// Check for unsafe operations
	if err := ge.checkUnsafeOperations(code); err != nil {
		return fmt.Errorf("unsafe operation detected: %w", err)
	}

	return nil
}

// extractAndRemoveImports extracts import statements from code and returns them along with cleaned code
func (ge *GoExecutor) extractAndRemoveImports(code string) ([]string, string) {
	var extractedImports []string
	var cleanedLines []string
	
	// Regular expressions for different import formats
	importLineRegex := regexp.MustCompile(`^\s*import\s+"([^"]+)"\s*$`)
	importAliasRegex := regexp.MustCompile(`^\s*import\s+\w+\s+"([^"]+)"\s*$`)
	importBlockStart := regexp.MustCompile(`^\s*import\s*\(\s*$`)
	importBlockEnd := regexp.MustCompile(`^\s*\)\s*$`)
	importInBlock := regexp.MustCompile(`^\s*(?:\w+\s+)?"([^"]+)"\s*$`)
	
	lines := strings.Split(code, "\n")
	inImportBlock := false
	
	for _, line := range lines {
		// Check if we're starting an import block
		if importBlockStart.MatchString(line) {
			inImportBlock = true
			continue
		}
		
		// Check if we're ending an import block
		if inImportBlock && importBlockEnd.MatchString(line) {
			inImportBlock = false
			continue
		}
		
		// Extract imports from within import block
		if inImportBlock {
			if matches := importInBlock.FindStringSubmatch(line); len(matches) > 1 {
				extractedImports = append(extractedImports, matches[1])
			}
			continue
		}
		
		// Check for single-line import
		if matches := importLineRegex.FindStringSubmatch(line); len(matches) > 1 {
			extractedImports = append(extractedImports, matches[1])
			continue
		}
		
		// Check for single-line import with alias
		if matches := importAliasRegex.FindStringSubmatch(line); len(matches) > 1 {
			extractedImports = append(extractedImports, matches[1])
			continue
		}
		
		// If not an import line, add to cleaned code
		cleanedLines = append(cleanedLines, line)
	}
	
	return extractedImports, strings.Join(cleanedLines, "\n")
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
	// Always forbidden operations regardless of settings
	alwaysUnsafePatterns := []string{
		`unsafe\.`,        // unsafe package usage
		`syscall\.`,       // direct syscalls  
		`os\.Exit`,        // program termination
		`panic\s*\(`,      // panic calls (except in controlled scenarios)
		`recover\s*\(`,    // recover calls
		`exec\.Command`,   // command execution
		`exec\.CommandContext`, // command execution with context
		`plugin\.`,        // plugin loading
		`runtime\.SetFinalizer`, // finalizer manipulation
		`reflect\.ValueOf\([^)]+\)\.UnsafePointer`, // unsafe pointer access
	}

	unsafePatterns := alwaysUnsafePatterns

	if !ge.enableFileSystem {
		unsafePatterns = append(unsafePatterns,
			`os\.Create`, `os\.Open`, `os\.OpenFile`, `os\.Remove`, `os\.RemoveAll`,
			`os\.Mkdir`, `os\.MkdirAll`, `os\.Chmod`, `os\.Chown`,
			`os\.Rename`, `os\.Link`, `os\.Symlink`,
			`ioutil\.WriteFile`, `ioutil\.ReadFile`, `ioutil\.TempFile`,
			`filepath\.Walk`, `filepath\.WalkDir`,
			`os\.ReadDir`, `os\.ReadFile`, `os\.WriteFile`,
		)
	}

	if !ge.enableNetworking {
		unsafePatterns = append(unsafePatterns,
			`net\.Dial`, `net\.Listen`, `net\.ListenPacket`,
			`http\.Get`, `http\.Post`, `http\.NewRequest`,
			`http\.ListenAndServe`, `http\.Serve`,
			`smtp\.`, `rpc\.`, `mail\.`,
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
// parseCodeToAST parses the code and returns the AST
func parseCodeToAST(code string) (*ast.File, error) {
	fset := token.NewFileSet()
	wrappedCode := fmt.Sprintf(`package main
func _() {
%s
}`, code)
	return parser.ParseFile(fset, "", wrappedCode, parser.ParseComments)
}

// findFunctionBody finds the wrapped function body in the AST
func findFunctionBody(node *ast.File) *ast.BlockStmt {
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "_" {
			return fn.Body
		}
	}
	return nil
}

// processAssignment extracts new variables from assignment statements
func processAssignment(x *ast.AssignStmt, existingVars, foundVars map[string]bool, newVars *[]string) {
	for _, lhs := range x.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
			if !existingVars[ident.Name] && !foundVars[ident.Name] {
				foundVars[ident.Name] = true
				*newVars = append(*newVars, ident.Name)
			}
		}
	}
}

// processVarDeclaration extracts new variables from var declarations
func processVarDeclaration(x *ast.DeclStmt, existingVars, foundVars map[string]bool, newVars *[]string) {
	genDecl, ok := x.Decl.(*ast.GenDecl)
	if !ok || genDecl.Tok != token.VAR {
		return
	}
	
	for _, spec := range genDecl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range valueSpec.Names {
				if name.Name != "_" && !existingVars[name.Name] && !foundVars[name.Name] {
					foundVars[name.Name] = true
					*newVars = append(*newVars, name.Name)
				}
			}
		}
	}
}

// VariableInfo contains information about a variable
type VariableInfo struct {
	Name       string
	IsVarDecl  bool // true for var declarations, false for := assignments
}

func (ge *GoExecutor) extractNewVariables(code string) ([]string, map[string]bool, error) {
	node, err := parseCodeToAST(code)
	if err != nil {
		return nil, nil, nil // Return empty on parse error
	}

	funcBody := findFunctionBody(node)
	if funcBody == nil {
		return nil, nil, nil
	}

	// Initialize tracking maps
	existingVars := make(map[string]bool)
	for name := range ge.variables {
		existingVars[name] = true
	}
	foundVars := make(map[string]bool)
	varDeclVars := make(map[string]bool) // Track which vars come from var declarations
	var newVars []string

	// Visit all nodes to find variable declarations
	ast.Inspect(funcBody, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			// Don't descend into function literals - their variables are local
			return false
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				// Process := assignments
				processAssignment(x, existingVars, foundVars, &newVars)
			} else if x.Tok == token.ASSIGN {
				// Process = assignments to track undefined variables that need declaration
				for _, lhs := range x.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
						if !existingVars[ident.Name] && !foundVars[ident.Name] {
							foundVars[ident.Name] = true
							newVars = append(newVars, ident.Name)
						}
					}
				}
			}
		case *ast.DeclStmt:
			// Process var declarations but mark them differently
			genDecl, ok := x.Decl.(*ast.GenDecl)
			if ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if name.Name != "_" && !existingVars[name.Name] && !foundVars[name.Name] {
								foundVars[name.Name] = true
								newVars = append(newVars, name.Name)
								varDeclVars[name.Name] = true // Mark as var declaration
							}
						}
					}
				}
			}
		}
		return true
	})

	return newVars, varDeclVars, nil
}

// transformStandaloneFunctions transforms standalone function definitions to variable assignments
func (ge *GoExecutor) transformStandaloneFunctions(code string) string {
	// Pattern to match standalone function definitions like: func isPrime(n int) bool { ... }
	// This now handles functions with or without return types
	funcDefPattern := regexp.MustCompile(`(?ms)^(\s*)func\s+(\w+)\s*\(([^)]*)\)\s*([^{]*?)\s*\{`)
	
	// Find all function definitions
	allMatches := funcDefPattern.FindAllStringSubmatchIndex(code, -1)
	
	// Process in reverse order to avoid position shifts
	for i := len(allMatches) - 1; i >= 0; i-- {
		match := funcDefPattern.FindStringSubmatch(code[allMatches[i][0]:])
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
		
		// Transform to variable assignment format
		oldDecl := fmt.Sprintf(`%sfunc %s(%s) %s {`, indent, funcName, params, returnType)
		newDecl := fmt.Sprintf(`%s%s := func(%s) %s {`, indent, funcName, params, returnType)
		
		// Replace in the code
		code = code[:startPos] + strings.Replace(code[startPos:], oldDecl, newDecl, 1)
	}
	
	return code
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
// and var declarations to simple assignments
func (ge *GoExecutor) transformVariableDeclarations(code string, declaredVars map[string]bool) string {
	// For now, use a simple regex-based approach
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		// Match var name = value pattern
		if matches := regexp.MustCompile(`^\s*var\s+(\w+)\s*=\s*(.+)$`).FindStringSubmatch(line); len(matches) == 3 {
			varName := matches[1]
			value := matches[2]
			// Transform to simple assignment
			lines[i] = fmt.Sprintf("%s = %s", varName, value)
		} else if matches := regexp.MustCompile(`^\s*(\w+)\s*:=\s*(.+)$`).FindStringSubmatch(line); len(matches) == 3 {
			// Match variable := value pattern
			varName := matches[1]
			if declaredVars[varName] {
				// Replace := with =
				lines[i] = regexp.MustCompile(`:=`).ReplaceAllString(line, "=")
			}
		}
	}
	return strings.Join(lines, "\n")
}

// isVariableInitializedWithLiteral checks if a variable is initialized with a literal value
func (ge *GoExecutor) isVariableInitializedWithLiteral(varName, code string) bool {
	// Check for literal number assignment
	literalPatterns := []string{
		fmt.Sprintf(`\b%s\s*:=\s*\d+`, varName),           // integer literal
		fmt.Sprintf(`\b%s\s*:=\s*\d+\.\d+`, varName),    // float literal  
		fmt.Sprintf(`\b%s\s*:=\s*"[^"]*"`, varName),      // string literal
		fmt.Sprintf(`\b%s\s*:=\s*'[^']*'`, varName),        // char literal
		fmt.Sprintf(`\b%s\s*:=\s*(?:true|false)`, varName), // bool literal
	}
	
	for _, pattern := range literalPatterns {
		if matched, _ := regexp.MatchString(pattern, code); matched {
			return true
		}
	}
	
	return false
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
}`
}

// wrapArithmeticOperations wraps variables in type conversion functions for arithmetic
// isVariableFunction checks if a variable is a function
func (ge *GoExecutor) isVariableFunction(varName, code string) bool {
	funcPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, varName))
	varFuncPattern := regexp.MustCompile(fmt.Sprintf(`\bvar\s+%s\s+func\s*\(`, varName))
	return funcPattern.MatchString(code) || varFuncPattern.MatchString(code)
}

// getConversionFunction determines the appropriate type conversion function
func getConversionFunction(value interface{}) string {
	switch v := value.(type) {
	case int, int32, int64:
		return "toInt"
	case float64, float32:
		// Check if it's actually an integer stored as float64 (common with JSON)
		if float64(int(v.(float64))) == v.(float64) {
			return "toInt"
		}
		return "toFloat64"
	case string:
		return "toString"
	case bool:
		return "toBool"
	case func(...interface{}) interface{}:
		return ""
	default:
		return ""
	}
}

// wrapVariableInForLoop handles variable wrapping in for loops
func wrapVariableInForLoop(line, varName, convFunc string) (string, bool) {
	if !strings.Contains(line, "for") || !strings.Contains(line, ":=") {
		return line, false
	}
	
	forLoopRegex := regexp.MustCompile(`^(\s*for\s+\w+\s*:=\s*[^;]+;\s*)([^;]+)(\s*;\s*[^{]+)(.*)$`)
	matches := forLoopRegex.FindStringSubmatch(line)
	if len(matches) < 4 {
		return line, false
	}
	
	condition := matches[2]
	if !strings.Contains(condition, varName) {
		return line, false
	}
	
	wrappedCondition := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, varName)).ReplaceAllString(condition, convFunc+"("+varName+")")
	return matches[1] + wrappedCondition + matches[3] + matches[4], true
}

// wrapVariableInLine handles variable wrapping in a single line
func wrapVariableInLine(line, varName, convFunc string) string {
	// Skip var declarations - don't wrap them
	if strings.HasPrefix(strings.TrimSpace(line), "var ") {
		return line
	}
	
	// Check for compound assignment operators (+=, -=, etc.)
	compoundRegex := regexp.MustCompile(fmt.Sprintf(`^(\s*%s\s*)([+\-*/%%]=)\s*(.+)$`, varName))
	if matches := compoundRegex.FindStringSubmatch(line); len(matches) == 4 {
		op := matches[2][:1]
		return fmt.Sprintf("%s= %s(%s) %s %s", matches[1], convFunc, varName, op, matches[3])
	}
	
	// Check for regular assignment
	assignmentRegex := regexp.MustCompile(fmt.Sprintf(`^(\s*%s\s*=\s*)(.+)$`, varName))
	if matches := assignmentRegex.FindStringSubmatch(line); len(matches) == 3 {
		rhs := matches[2]
		wrappedRhs := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, varName)).ReplaceAllString(rhs, convFunc+"("+varName+")")
		return matches[1] + wrappedRhs
	}
	
	// Check if variable is used in expressions
	if regexp.MustCompile(fmt.Sprintf(`\b%s\s*[+\-*/%%<>=]`, varName)).MatchString(line) ||
		regexp.MustCompile(fmt.Sprintf(`[+\-*/%%<>=]\s*%s\b`, varName)).MatchString(line) {
		// Skip if this line contains Printf, Sprintf, Fprintf, etc.
		if strings.Contains(line, "Printf") || strings.Contains(line, "Sprintf") || strings.Contains(line, "Fprintf") {
			return line
		}
		
		// Replace variable occurrences but not function calls
		varPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, varName))
		return varPattern.ReplaceAllStringFunc(line, func(match string) string {
			matchIndex := strings.Index(line, match)
			if matchIndex >= 0 && matchIndex+len(match) < len(line) {
				afterMatch := line[matchIndex+len(match):]
				if strings.TrimSpace(afterMatch) != "" && strings.TrimSpace(afterMatch)[0] == '(' {
					return match // Function call, don't wrap
				}
			}
			return convFunc + "(" + match + ")"
		})
	}
	
	return line
}

func (ge *GoExecutor) wrapArithmeticOperations(code string, existingVars map[string]interface{}) string {
	for varName, value := range existingVars {
		if value == nil {
			continue
		}

		// Skip function variables
		if ge.isVariableFunction(varName, code) {
			continue
		}

		// Determine the appropriate conversion function
		convFunc := getConversionFunction(value)
		if convFunc == "" {
			continue
		}

		// Process each line
		lines := strings.Split(code, "\n")
		for i, line := range lines {
			// Try for loop wrapping first
			if newLine, handled := wrapVariableInForLoop(line, varName, convFunc); handled {
				lines[i] = newLine
				continue
			}
			
			// Otherwise handle regular line wrapping
			lines[i] = wrapVariableInLine(line, varName, convFunc)
		}
		code = strings.Join(lines, "\n")
	}

	return code
}

// detectUsedImports analyzes code to determine which imports are actually used
func (ge *GoExecutor) detectUsedImports(code string, availableImports []string) []string {
	// Map of package names to their import paths
	packageMap := map[string]string{
		"json":     "encoding/json",
		"base64":   "encoding/base64",
		"csv":      "encoding/csv",
		"md5":      "crypto/md5",
		"sha1":     "crypto/sha1",
		"gzip":     "compress/gzip",
		"zip":      "archive/zip",
		"url":      "net/url",
		"filepath": "path/filepath",
		"path":     "path",
		"rand":     "math/rand",
		"math":     "math",
		"strings":  "strings",
		"strconv":  "strconv",
		"time":     "time",
		"regexp":   "regexp",
		"sort":     "sort",
		"unicode":  "unicode",
		"utf8":     "unicode/utf8",
		"bytes":    "bytes",
		"bufio":    "bufio",
		"io":       "io",
		"list":     "container/list",
		"heap":     "container/heap",
		"reflect":  "reflect",
		"os":       "os",
		"crypto":   "crypto",
		"sha256":   "crypto/sha256",
		"cmplx":    "math/cmplx",
		"big":      "math/big",
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
// prepareVariablesAndCode extracts variables and transforms code
func (ge *GoExecutor) prepareVariablesAndCode(code string) ([]string, map[string]bool, string, map[string]bool) {
	// Transform standalone functions first
	code = ge.transformStandaloneFunctions(code)
	
	newVars, varDeclVars, _ := ge.extractNewVariables(code)
	code = ge.transformFunctionVariables(code)
	
	// Only mark existing variables as declared, not new ones
	// This ensures new variables keep their := syntax
	declaredVars := make(map[string]bool)
	for name := range ge.variables {
		declaredVars[name] = true
	}
	// Don't add newVars to declaredVars - let them use :=
	
	transformedCode := ge.transformVariableDeclarations(code, declaredVars)
	
	return newVars, varDeclVars, transformedCode, declaredVars
}

// prepareWrappingVariables builds the variable map for arithmetic wrapping
func (ge *GoExecutor) prepareWrappingVariables(code string, newVars []string) map[string]interface{} {
	allVarsForWrapping := make(map[string]interface{})
	for k, v := range ge.variables {
		allVarsForWrapping[k] = v
	}
	
	for _, varName := range newVars {
		funcInitPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, varName))
		if funcInitPattern.MatchString(code) {
			continue
		}
		
		intInitPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*(\d+)\b`, varName))
		if matches := intInitPattern.FindStringSubmatch(code); len(matches) > 1 {
			allVarsForWrapping[varName] = 0
		} else {
			allVarsForWrapping[varName] = 0
		}
	}
	
	return allVarsForWrapping
}

// buildImportList creates the import statements
func (ge *GoExecutor) buildImportList(transformedCode string, authorizedPackages []string) []string {
	allAvailablePackages := append(ge.authorizedPackages, authorizedPackages...)
	usedImports := ge.detectUsedImports(transformedCode, allAvailablePackages)
	
	var imports []string
	for _, pkg := range usedImports {
		if pkg == "fmt" {
			continue
		}
		imports = append(imports, fmt.Sprintf(`"%s"`, pkg))
	}
	imports = append(imports, `__fmt__ "fmt"`)
	
	return imports
}

// buildExistingVariableDeclarations creates declarations for existing variables
func (ge *GoExecutor) buildExistingVariableDeclarations() []string {
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
	return variableDeclarations
}

// identifyFunctionVariables detects which variables are functions
func (ge *GoExecutor) identifyFunctionVariables(code, transformedCode string, newVars []string, varDeclVars map[string]bool) (map[string]string, []string) {
	funcVarTypes := make(map[string]string)
	var nonFuncDeclarations []string
	
	for _, varName := range newVars {
		// Don't skip var declarations anymore - we need interface{} declarations for them too
		
		funcPattern := regexp.MustCompile(fmt.Sprintf(`(?s)\b%s\s*:=\s*func\s*\((.*?)\)\s*(.*?)\s*\{`, regexp.QuoteMeta(varName)))
		varFuncPattern := regexp.MustCompile(fmt.Sprintf(`(?m)\bvar\s+%s\s+func\s*\((.*?)\)\s*(.*?)$`, regexp.QuoteMeta(varName)))
		
		isFuncVar := false
		if funcPattern.MatchString(code) || funcPattern.MatchString(transformedCode) {
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
			isFuncVar = true
			if matches := varFuncPattern.FindStringSubmatch(transformedCode); len(matches) > 2 {
				params := matches[1]
				returns := strings.TrimSpace(matches[2])
				funcVarTypes[varName] = fmt.Sprintf("func(%s) %s", params, returns)
			}
		}
		
		if !isFuncVar {
			// Pre-declare variables to support auto-declaration behavior
			// This allows code like "result = 5" without explicit declaration
			nonFuncDeclarations = append(nonFuncDeclarations,
				fmt.Sprintf("var %s interface{}", varName))
		}
	}
	
	return funcVarTypes, nonFuncDeclarations
}

// buildVariableCaptures creates the variable capture map
func (ge *GoExecutor) buildVariableCaptures(newVars []string, funcVarTypes map[string]string, transformedCode string) (string, []string) {
	// Only capture existing variables from previous executions
	// New variables will be handled differently
	var captureCode []string
	var allCapturedVars []string
	
	// Only add existing variables from previous executions
	for name := range ge.variables {
		// Skip function variables
		funcPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, name))
		varFuncPattern := regexp.MustCompile(fmt.Sprintf(`\bvar\s+%s\s+func\s*\(`, name))
		if !funcPattern.MatchString(transformedCode) && !varFuncPattern.MatchString(transformedCode) {
			captureCode = append(captureCode,
				fmt.Sprintf(`"%s": %s`, name, name))
			allCapturedVars = append(allCapturedVars, name)
		}
	}
	
	// Track new variables separately for later capture
	var newNonFuncVars []string
	for _, name := range newVars {
		// Skip function variables
		if _, isFunc := funcVarTypes[name]; isFunc {
			continue
		}
		funcPattern := regexp.MustCompile(fmt.Sprintf(`\b%s\s*:=\s*func\s*\(`, name))
		varFuncPattern := regexp.MustCompile(fmt.Sprintf(`\bvar\s+%s\s+func\s*\(`, name))
		if !funcPattern.MatchString(transformedCode) && !varFuncPattern.MatchString(transformedCode) {
			newNonFuncVars = append(newNonFuncVars, name)
		}
	}
	
	var captureStr string
	if len(captureCode) == 0 {
		captureStr = ""
	} else if len(captureCode) == 1 {
		captureStr = captureCode[0]
	} else {
		captureStr = "\n\t\t" + strings.Join(captureCode, ",\n\t\t") + ",\n\t"
	}
	
	return captureStr, newNonFuncVars
}

func (ge *GoExecutor) buildProgram(code string, authorizedPackages []string) (string, error) {
	// Prepare variables and transform code
	newVars, varDeclVars, transformedCode, _ := ge.prepareVariablesAndCode(code)
	
	// Prepare wrapping variables and wrap arithmetic operations
	allVarsForWrapping := ge.prepareWrappingVariables(code, newVars)
	transformedCode = ge.wrapArithmeticOperations(transformedCode, allVarsForWrapping)
	
	// Build import list
	imports := ge.buildImportList(transformedCode, authorizedPackages)
	
	// Build existing variable declarations
	variableDeclarations := ge.buildExistingVariableDeclarations()
	
	// Identify function variables and build new variable declarations
	funcVarTypes, nonFuncDeclarations := ge.identifyFunctionVariables(code, transformedCode, newVars, varDeclVars)
	variableDeclarations = append(variableDeclarations, nonFuncDeclarations...)
	
	// Build variable captures
	variableCapturesStr, newNonFuncVars := ge.buildVariableCaptures(newVars, funcVarTypes, transformedCode)
	
	// Build list of new variables to track
	var newVarsList []string
	var newVarCases []string
	for _, varName := range newNonFuncVars {
		newVarsList = append(newVarsList, fmt.Sprintf(`"%s"`, varName))
		// Generate case for capturing this variable
		newVarCases = append(newVarCases, fmt.Sprintf(`case "%s":
			variables["%s"] = %s`, varName, varName, varName))
	}
	newVarsStr := strings.Join(newVarsList, ", ")
	newVarCasesStr := strings.Join(newVarCases, "\n\t\t")

	// Build the complete program
	program := fmt.Sprintf(`package main

import (
	%s
)

%s

// Global print output buffer
var printBuffer bytes.Buffer

// List of new variables to track
var newVariableNames = []string{%s}

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
		"variables": map[string]interface{}{}, // Variables will be captured at end of execution
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
	vars := map[string]interface{}{%s}
	// Note: new variables defined in code are captured after execution
	return vars
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
	
	// Capture new variables defined in this execution
	for _, varName := range newVariableNames {
		// Capture each new variable
		switch varName {
		%s
		}
	}
	
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
		newVarsStr,
		ge.generateTypeHelpers(),
		variableCapturesStr,
		transformedCode,
		newVarCasesStr,
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
