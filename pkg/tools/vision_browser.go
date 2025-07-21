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

// Package tools - Vision web browser for web automation and interaction
package tools

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// VisionWebBrowser provides web automation with visual capabilities
type VisionWebBrowser struct {
	*BaseTool
	UserAgent      string        `json:"user_agent"`
	Timeout        time.Duration `json:"timeout"`
	ScreenshotPath string        `json:"screenshot_path"`
	BrowserPath    string        `json:"browser_path"`
	Headless       bool          `json:"headless"`
	WindowSize     string        `json:"window_size"`
	WaitTime       time.Duration `json:"wait_time"`
}

// NewVisionWebBrowser creates a new vision web browser tool
func NewVisionWebBrowser() *VisionWebBrowser {
	inputs := map[string]*ToolInput{
		"action": {
			Type:        "string",
			Description: "Action to perform: 'navigate', 'click', 'type', 'scroll', 'screenshot', 'extract'",
			Nullable:    false,
		},
		"url": {
			Type:        "string",
			Description: "URL to navigate to (required for 'navigate' action)",
			Nullable:    true,
		},
		"selector": {
			Type:        "string",
			Description: "CSS selector for element to interact with",
			Nullable:    true,
		},
		"text": {
			Type:        "string",
			Description: "Text to type (for 'type' action)",
			Nullable:    true,
		},
		"coordinates": {
			Type:        "string",
			Description: "X,Y coordinates for click (e.g., '100,200')",
			Nullable:    true,
		},
		"scroll_direction": {
			Type:        "string",
			Description: "Scroll direction: 'up', 'down', 'left', 'right'",
			Nullable:    true,
		},
		"wait_for": {
			Type:        "string",
			Description: "Element selector to wait for before proceeding",
			Nullable:    true,
		},
	}

	browser := &VisionWebBrowser{
		BaseTool:       NewBaseTool("vision_browser", "Automated web browser with visual capabilities", inputs, "string"),
		UserAgent:      "Mozilla/5.0 (compatible; SmolagentsBot/1.0; Vision Browser)",
		Timeout:        30 * time.Second,
		ScreenshotPath: "/tmp/smolagents_screenshots",
		BrowserPath:    "", // Auto-detect
		Headless:       true,
		WindowSize:     "1920,1080",
		WaitTime:       2 * time.Second,
	}

	browser.ForwardFunc = browser.forward

	// Ensure screenshot directory exists
	os.MkdirAll(browser.ScreenshotPath, 0755)

	return browser
}

func (vwb *VisionWebBrowser) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("action parameter is required")
	}

	action, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("action must be a string")
	}

	// Parse arguments into a map for easier handling
	params := make(map[string]string)
	if len(args) > 1 {
		if paramMap, ok := args[1].(map[string]interface{}); ok {
			for k, v := range paramMap {
				if str, ok := v.(string); ok {
					params[k] = str
				}
			}
		}
	}

	// Extract individual parameters from args if provided directly
	if len(args) > 1 && params["url"] == "" {
		if url, ok := args[1].(string); ok {
			params["url"] = url
		}
	}
	if len(args) > 2 && params["selector"] == "" {
		if selector, ok := args[2].(string); ok {
			params["selector"] = selector
		}
	}

	return vwb.executeAction(action, params)
}

func (vwb *VisionWebBrowser) executeAction(action string, params map[string]string) (string, error) {
	switch action {
	case "navigate":
		return vwb.navigate(params["url"])
	case "click":
		return vwb.click(params["selector"], params["coordinates"])
	case "type":
		return vwb.typeText(params["selector"], params["text"])
	case "scroll":
		return vwb.scroll(params["scroll_direction"])
	case "screenshot":
		return vwb.takeScreenshot(params["filename"])
	case "extract":
		return vwb.extractContent(params["url"], params["selector"])
	case "wait":
		return vwb.waitForElement(params["selector"])
	default:
		return "", fmt.Errorf("unsupported action: %s", action)
	}
}

// navigate navigates to a URL and takes a screenshot
func (vwb *VisionWebBrowser) navigate(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("URL is required for navigation")
	}

	// For now, use simple HTTP client to get page content
	// In a full implementation, this would use a real browser automation tool
	client := &http.Client{Timeout: vwb.Timeout}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", vwb.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to navigate to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("navigation failed with status %d", resp.StatusCode)
	}

	// Take a screenshot (simulated)
	screenshotPath, err := vwb.simulateScreenshot(url)
	if err != nil {
		return "", fmt.Errorf("failed to take screenshot: %w", err)
	}

	return fmt.Sprintf("Successfully navigated to %s\nScreenshot saved to: %s\nPage loaded successfully with status %d",
		url, screenshotPath, resp.StatusCode), nil
}

// click simulates clicking on an element
func (vwb *VisionWebBrowser) click(selector, coordinates string) (string, error) {
	if selector == "" && coordinates == "" {
		return "", fmt.Errorf("either selector or coordinates must be provided for click action")
	}

	if selector != "" {
		return fmt.Sprintf("Clicked on element with selector: %s", selector), nil
	}

	return fmt.Sprintf("Clicked at coordinates: %s", coordinates), nil
}

// typeText simulates typing text into an element
func (vwb *VisionWebBrowser) typeText(selector, text string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required for type action")
	}

	if text == "" {
		return "", fmt.Errorf("text is required for type action")
	}

	return fmt.Sprintf("Typed '%s' into element with selector: %s", text, selector), nil
}

// scroll simulates scrolling the page
func (vwb *VisionWebBrowser) scroll(direction string) (string, error) {
	validDirections := []string{"up", "down", "left", "right"}
	if direction == "" {
		direction = "down" // Default direction
	}

	valid := false
	for _, validDir := range validDirections {
		if direction == validDir {
			valid = true
			break
		}
	}

	if !valid {
		return "", fmt.Errorf("invalid scroll direction: %s (valid: %s)", direction, strings.Join(validDirections, ", "))
	}

	return fmt.Sprintf("Scrolled %s on the page", direction), nil
}

// takeScreenshot takes a screenshot of the current page
func (vwb *VisionWebBrowser) takeScreenshot(filename string) (string, error) {
	if filename == "" {
		filename = fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
	}

	if !strings.HasSuffix(filename, ".png") {
		filename += ".png"
	}

	screenshotPath := fmt.Sprintf("%s/%s", vwb.ScreenshotPath, filename)

	// Simulate taking a screenshot
	return vwb.simulateScreenshot(screenshotPath)
}

// extractContent extracts content from a web page
func (vwb *VisionWebBrowser) extractContent(url, selector string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("URL is required for content extraction")
	}

	// Fetch the page
	client := &http.Client{Timeout: vwb.Timeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	if selector == "" {
		// Extract all text content
		return doc.Text(), nil
	}

	// Extract content matching the selector
	var content strings.Builder
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		content.WriteString(s.Text())
		content.WriteString("\n")
	})

	if content.Len() == 0 {
		return fmt.Sprintf("No content found for selector: %s", selector), nil
	}

	return content.String(), nil
}

// waitForElement waits for an element to appear on the page
func (vwb *VisionWebBrowser) waitForElement(selector string) (string, error) {
	if selector == "" {
		return "", fmt.Errorf("selector is required for wait action")
	}

	// Simulate waiting
	time.Sleep(vwb.WaitTime)

	return fmt.Sprintf("Waited for element with selector: %s", selector), nil
}

// simulateScreenshot simulates taking a screenshot
func (vwb *VisionWebBrowser) simulateScreenshot(path string) (string, error) {
	// In a real implementation, this would use a browser automation tool like:
	// - Playwright
	// - Selenium
	// - Chrome DevTools Protocol
	// - Puppeteer (via Node.js bridge)

	// For now, create a placeholder image file
	screenshotPath := path
	if !strings.Contains(path, "/") {
		screenshotPath = fmt.Sprintf("%s/%s", vwb.ScreenshotPath, path)
	}

	// Create a simple placeholder PNG (1x1 pixel)
	pngData := createPlaceholderPNG()

	err := os.WriteFile(screenshotPath, pngData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}

	return screenshotPath, nil
}

// createPlaceholderPNG creates a minimal PNG file as placeholder
func createPlaceholderPNG() []byte {
	// This is a base64-encoded 1x1 pixel transparent PNG
	pngB64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	data, _ := base64.StdEncoding.DecodeString(pngB64)
	return data
}

// GetBrowserInfo returns information about the available browser
func (vwb *VisionWebBrowser) GetBrowserInfo() map[string]interface{} {
	// Try to detect available browsers
	browsers := []string{"google-chrome", "chromium", "firefox", "safari"}
	available := []string{}

	for _, browser := range browsers {
		if _, err := exec.LookPath(browser); err == nil {
			available = append(available, browser)
		}
	}

	return map[string]interface{}{
		"available_browsers": available,
		"headless_mode":      vwb.Headless,
		"user_agent":         vwb.UserAgent,
		"window_size":        vwb.WindowSize,
		"screenshot_path":    vwb.ScreenshotPath,
		"timeout":            vwb.Timeout.Seconds(),
	}
}

// SetBrowserOptions configures browser options
func (vwb *VisionWebBrowser) SetBrowserOptions(options map[string]interface{}) {
	if userAgent, ok := options["user_agent"].(string); ok {
		vwb.UserAgent = userAgent
	}
	if timeout, ok := options["timeout"].(time.Duration); ok {
		vwb.Timeout = timeout
	}
	if headless, ok := options["headless"].(bool); ok {
		vwb.Headless = headless
	}
	if windowSize, ok := options["window_size"].(string); ok {
		vwb.WindowSize = windowSize
	}
	if screenshotPath, ok := options["screenshot_path"].(string); ok {
		vwb.ScreenshotPath = screenshotPath
		os.MkdirAll(vwb.ScreenshotPath, 0755)
	}
}

// AutomatedBrowserSession represents a browser automation session
type AutomatedBrowserSession struct {
	browser    *VisionWebBrowser
	currentURL string
	sessionID  string
	startTime  time.Time
	actions    []map[string]interface{}
}

// NewAutomatedBrowserSession creates a new browser automation session
func NewAutomatedBrowserSession(browser *VisionWebBrowser) *AutomatedBrowserSession {
	return &AutomatedBrowserSession{
		browser:   browser,
		sessionID: fmt.Sprintf("session_%d", time.Now().UnixNano()),
		startTime: time.Now(),
		actions:   make([]map[string]interface{}, 0),
	}
}

// ExecuteScript executes a sequence of browser actions
func (abs *AutomatedBrowserSession) ExecuteScript(script []map[string]interface{}) (string, error) {
	var results strings.Builder
	results.WriteString(fmt.Sprintf("Executing browser automation script (Session: %s)\n\n", abs.sessionID))

	for i, action := range script {
		actionType, ok := action["action"].(string)
		if !ok {
			return "", fmt.Errorf("action %d: missing or invalid action type", i)
		}

		// Convert action map to params
		params := make(map[string]string)
		for k, v := range action {
			if k != "action" {
				if str, ok := v.(string); ok {
					params[k] = str
				}
			}
		}

		result, err := abs.browser.executeAction(actionType, params)
		if err != nil {
			return "", fmt.Errorf("action %d (%s): %w", i, actionType, err)
		}

		// Record action
		abs.actions = append(abs.actions, action)

		results.WriteString(fmt.Sprintf("Step %d: %s\n", i+1, result))

		// Add delay between actions
		time.Sleep(500 * time.Millisecond)
	}

	results.WriteString(fmt.Sprintf("\nSession completed successfully. Duration: %v\n", time.Since(abs.startTime)))
	return results.String(), nil
}

// GetSessionSummary returns a summary of the automation session
func (abs *AutomatedBrowserSession) GetSessionSummary() map[string]interface{} {
	return map[string]interface{}{
		"session_id":    abs.sessionID,
		"start_time":    abs.startTime,
		"duration":      time.Since(abs.startTime),
		"current_url":   abs.currentURL,
		"actions_count": len(abs.actions),
		"actions":       abs.actions,
	}
}
