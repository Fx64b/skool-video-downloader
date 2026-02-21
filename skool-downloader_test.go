package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestExtractLoomURLs(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected []string
	}{
		{
			name:     "Empty HTML",
			html:     "",
			expected: []string{},
		},
		{
			name:     "No Loom URLs",
			html:     "<html><body>No videos here</body></html>",
			expected: []string{},
		},
		{
			name:     "Single share URL",
			html:     `<html><body><a href="https://www.loom.com/share/abc123">Video</a></body></html>`,
			expected: []string{"https://www.loom.com/share/abc123"},
		},
		{
			name:     "Single share URL without www",
			html:     `<html><body><a href="https://loom.com/share/xyz789">Video</a></body></html>`,
			expected: []string{"https://loom.com/share/xyz789"},
		},
		{
			name:     "Single embed URL",
			html:     `<html><body><iframe src="https://www.loom.com/embed/def456"></iframe></body></html>`,
			expected: []string{"https://www.loom.com/share/def456"},
		},
		{
			name:     "Multiple URLs",
			html:     `<html><body><a href="https://www.loom.com/share/abc123">Video1</a><a href="https://loom.com/share/xyz789">Video2</a></body></html>`,
			expected: []string{"https://www.loom.com/share/abc123", "https://loom.com/share/xyz789"},
		},
		{
			name:     "Duplicate URLs",
			html:     `<html><body><a href="https://www.loom.com/share/abc123">Video1</a><a href="https://www.loom.com/share/abc123">Video2</a></body></html>`,
			expected: []string{"https://www.loom.com/share/abc123"},
		},
		{
			name:     "Mix of share and embed URLs",
			html:     `<html><body><a href="https://www.loom.com/share/abc123">Video1</a><iframe src="https://loom.com/embed/def456"></iframe></body></html>`,
			expected: []string{"https://www.loom.com/share/abc123", "https://www.loom.com/share/def456"},
		},
		{
			name:     "Embed and share of same video",
			html:     `<html><body><a href="https://www.loom.com/share/abc123">Video1</a><iframe src="https://loom.com/embed/abc123"></iframe></body></html>`,
			expected: []string{"https://www.loom.com/share/abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLoomURLs(tt.html)
			// Handle nil vs empty slice comparison
			if len(result) == 0 && len(tt.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("extractLoomURLs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  int64
		shouldErr bool
	}{
		{
			name:      "Valid positive number",
			input:     "12345",
			expected:  12345,
			shouldErr: false,
		},
		{
			name:      "Valid zero",
			input:     "0",
			expected:  0,
			shouldErr: false,
		},
		{
			name:      "Valid negative number",
			input:     "-999",
			expected:  -999,
			shouldErr: false,
		},
		{
			name:      "Invalid string",
			input:     "abc",
			expected:  0,
			shouldErr: true,
		},
		{
			name:      "Empty string",
			input:     "",
			expected:  0,
			shouldErr: true,
		},
		{
			name:      "Large number",
			input:     "9223372036854775807", // max int64
			expected:  9223372036854775807,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInt64(tt.input)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("parseInt64(%q) expected error, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseInt64(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("parseInt64(%q) = %d, want %d", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestConvertJSONToNetscapeCookies(t *testing.T) {
	// Create a temporary JSON cookies file
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "cookies.json")

	jsonContent := `[
		{
			"host": ".skool.com",
			"name": "test_cookie",
			"value": "test_value",
			"path": "/",
			"expiry": 1700000000,
			"isSecure": 1,
			"isHttpOnly": 1,
			"sameSite": 0
		},
		{
			"host": "www.skool.com",
			"name": "another_cookie",
			"value": "another_value",
			"path": "/path",
			"expiry": 1800000000,
			"isSecure": 0,
			"isHttpOnly": 0,
			"sameSite": 1
		}
	]`

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test conversion
	netscapeFile, err := convertJSONToNetscapeCookies(jsonFile)
	if err != nil {
		t.Fatalf("convertJSONToNetscapeCookies() error = %v", err)
	}
	defer func() {
		if err := os.Remove(netscapeFile); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	// Read the converted file
	content, err := os.ReadFile(netscapeFile)
	if err != nil {
		t.Fatalf("Failed to read converted file: %v", err)
	}

	contentStr := string(content)

	// Check for header
	if !contains(contentStr, "# Netscape HTTP Cookie File") {
		t.Error("Missing Netscape header")
	}

	// Check for cookie data
	if !contains(contentStr, "test_cookie") {
		t.Error("Missing test_cookie in output")
	}
	if !contains(contentStr, "test_value") {
		t.Error("Missing test_value in output")
	}
	if !contains(contentStr, "another_cookie") {
		t.Error("Missing another_cookie in output")
	}
	if !contains(contentStr, "TRUE") { // secure flag
		t.Error("Missing TRUE flag for secure cookie")
	}
	if !contains(contentStr, "FALSE") { // non-secure flag
		t.Error("Missing FALSE flag for non-secure cookie")
	}
}

func TestConvertJSONToNetscapeCookies_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(jsonFile, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := convertJSONToNetscapeCookies(jsonFile)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestConvertJSONToNetscapeCookies_NonexistentFile(t *testing.T) {
	_, err := convertJSONToNetscapeCookies("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestParseJSONCookies(t *testing.T) {
	jsonContent := []byte(`[
		{
			"host": ".example.com",
			"name": "cookie1",
			"value": "value1",
			"path": "/",
			"expiry": 1700000000,
			"isSecure": 1,
			"isHttpOnly": 1,
			"sameSite": 1
		},
		{
			"host": "www.example.com",
			"name": "cookie2",
			"value": "value2",
			"path": "/test",
			"expiry": 0,
			"isSecure": 0,
			"isHttpOnly": 0,
			"sameSite": 0
		}
	]`)

	cookies, err := parseJSONCookies(jsonContent)
	if err != nil {
		t.Fatalf("parseJSONCookies() error = %v", err)
	}

	if len(cookies) != 2 {
		t.Errorf("Expected 2 cookies, got %d", len(cookies))
	}

	// Check first cookie
	if cookies[0].Name != "cookie1" {
		t.Errorf("Expected name 'cookie1', got '%s'", cookies[0].Name)
	}
	if cookies[0].Value != "value1" {
		t.Errorf("Expected value 'value1', got '%s'", cookies[0].Value)
	}
	if cookies[0].Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", cookies[0].Domain)
	}
	if !cookies[0].Secure {
		t.Error("Expected Secure to be true")
	}
	if !cookies[0].HTTPOnly {
		t.Error("Expected HTTPOnly to be true")
	}
	if cookies[0].SameSite != network.CookieSameSiteLax {
		t.Errorf("Expected SameSite Lax, got %v", cookies[0].SameSite)
	}

	// Check second cookie
	if cookies[1].Name != "cookie2" {
		t.Errorf("Expected name 'cookie2', got '%s'", cookies[1].Name)
	}
	if cookies[1].Domain != "www.example.com" {
		t.Errorf("Expected domain 'www.example.com', got '%s'", cookies[1].Domain)
	}
	if cookies[1].Secure {
		t.Error("Expected Secure to be false")
	}
	if cookies[1].HTTPOnly {
		t.Error("Expected HTTPOnly to be false")
	}
}

func TestParseJSONCookies_InvalidJSON(t *testing.T) {
	_, err := parseJSONCookies([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestParseNetscapeCookies(t *testing.T) {
	netscapeContent := []byte(`# Netscape HTTP Cookie File
# This is a comment
.example.com	TRUE	/	TRUE	1700000000	cookie1	value1
www.example.com	TRUE	/test	FALSE	0	cookie2	value2

# Another comment
.test.com	TRUE	/	TRUE	1800000000	cookie3	value3`)

	cookies, err := parseNetscapeCookies(netscapeContent)
	if err != nil {
		t.Fatalf("parseNetscapeCookies() error = %v", err)
	}

	if len(cookies) != 3 {
		t.Errorf("Expected 3 cookies, got %d", len(cookies))
	}

	// Check first cookie
	if cookies[0].Name != "cookie1" {
		t.Errorf("Expected name 'cookie1', got '%s'", cookies[0].Name)
	}
	if cookies[0].Value != "value1" {
		t.Errorf("Expected value 'value1', got '%s'", cookies[0].Value)
	}
	if cookies[0].Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", cookies[0].Domain)
	}
	if !cookies[0].Secure {
		t.Error("Expected Secure to be true")
	}

	// Check second cookie
	if cookies[1].Name != "cookie2" {
		t.Errorf("Expected name 'cookie2', got '%s'", cookies[1].Name)
	}
	if cookies[1].Path != "/test" {
		t.Errorf("Expected path '/test', got '%s'", cookies[1].Path)
	}
	if cookies[1].Secure {
		t.Error("Expected Secure to be false")
	}

	// Check third cookie
	if cookies[2].Name != "cookie3" {
		t.Errorf("Expected name 'cookie3', got '%s'", cookies[2].Name)
	}
}

func TestParseCookiesFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "cookies.json")

	jsonContent := `[
		{
			"host": ".example.com",
			"name": "test",
			"value": "value",
			"path": "/",
			"expiry": 1700000000,
			"isSecure": 1,
			"isHttpOnly": 1,
			"sameSite": 0
		}
	]`

	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cookies, err := parseCookiesFile(jsonFile)
	if err != nil {
		t.Fatalf("parseCookiesFile() error = %v", err)
	}

	if len(cookies) != 1 {
		t.Errorf("Expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", cookies[0].Name)
	}
}

func TestParseCookiesFile_Netscape(t *testing.T) {
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "cookies.txt")

	txtContent := `# Netscape HTTP Cookie File
.example.com	TRUE	/	TRUE	1700000000	test	value`

	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cookies, err := parseCookiesFile(txtFile)
	if err != nil {
		t.Fatalf("parseCookiesFile() error = %v", err)
	}

	if len(cookies) != 1 {
		t.Errorf("Expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", cookies[0].Name)
	}
}

func TestParseCookiesFile_AutoDetectJSON(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "cookies") // no extension

	jsonContent := `[
		{
			"host": ".example.com",
			"name": "test",
			"value": "value",
			"path": "/",
			"expiry": 1700000000,
			"isSecure": 1,
			"isHttpOnly": 1,
			"sameSite": 0
		}
	]`

	if err := os.WriteFile(file, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cookies, err := parseCookiesFile(file)
	if err != nil {
		t.Fatalf("parseCookiesFile() error = %v", err)
	}

	if len(cookies) != 1 {
		t.Errorf("Expected 1 cookie, got %d", len(cookies))
	}
}

func TestParseCookiesFile_NonexistentFile(t *testing.T) {
	_, err := parseCookiesFile("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestValidateConfig_NoURL(t *testing.T) {
	// This test will cause os.Exit(1), so we skip it in normal test runs
	// It's documented here for completeness
	t.Skip("Skipping test that calls os.Exit")
}

func TestValidateConfig_NoAuth(t *testing.T) {
	// This test will cause os.Exit(1), so we skip it in normal test runs
	// It's documented here for completeness
	t.Skip("Skipping test that calls os.Exit")
}

func TestFindBrowser_CustomAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	fakeBrowser := filepath.Join(tmpDir, "fake-browser")
	if err := os.WriteFile(fakeBrowser, []byte{}, 0755); err != nil {
		t.Fatalf("Failed to create fake browser file: %v", err)
	}

	path, err := findBrowser(fakeBrowser)
	if err != nil {
		t.Fatalf("findBrowser() error = %v", err)
	}
	if path != fakeBrowser {
		t.Errorf("findBrowser() = %v, want %v", path, fakeBrowser)
	}
}

func TestFindBrowser_InvalidCustomPath(t *testing.T) {
	_, err := findBrowser("/nonexistent/path/to/browser")
	if err == nil {
		t.Error("Expected error for nonexistent browser path, got nil")
	}
}

func TestFindBrowser_InvalidBareCommand(t *testing.T) {
	// A command name that is very unlikely to exist in PATH
	_, err := findBrowser("skool-nonexistent-browser-xyz")
	if err == nil {
		t.Error("Expected error for unknown browser command, got nil")
	}
}

func TestGetBrowserCandidates_NotEmpty(t *testing.T) {
	candidates := getBrowserCandidates()
	if len(candidates) == 0 {
		t.Error("getBrowserCandidates() returned an empty list")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
