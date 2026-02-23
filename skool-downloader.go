package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	defaultWaitTime  = 2
	defaultOutputDir = "downloads"
	defaultHeadless  = true
	browserTimeout   = 180 * time.Second
	initialWaitTime  = 3 * time.Second
	loginWaitTime    = 3 * time.Second
	skoolBaseURL     = "https://www.skool.com/"
	skoolLoginURL    = "https://www.skool.com/login"
)

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
)

// Colored log prefixes
const (
	prefixInfo     = colorBlue + "[INFO]" + colorReset
	prefixSuccess  = colorGreen + "[SUCCESS]" + colorReset
	prefixError    = colorRed + "[ERROR]" + colorReset
	prefixWarning  = colorYellow + "[WARNING]" + colorReset
	prefixAuth     = colorMagenta + "[AUTH]" + colorReset
	prefixDownload = colorCyan + "[DOWNLOAD]" + colorReset
)

// JSONCookie represents a cookie in the JSON format
type JSONCookie struct {
	Host       string `json:"host"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	Path       string `json:"path"`
	Expiry     int64  `json:"expiry"`
	IsSecure   int    `json:"isSecure"`
	IsHttpOnly int    `json:"isHttpOnly"`
	SameSite   int    `json:"sameSite"`
}

// Config holds application configuration
type Config struct {
	SkoolURL    string
	CookiesFile string
	Email       string
	Password    string
	OutputDir   string
	WaitTime    int
	Headless    bool
	BrowserPath string
}

func main() {
	printBanner()
	config := parseFlags()
	validateConfig(config)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	fmt.Println(prefixInfo, "Scraping videos from:", config.SkoolURL)

	// Scrape videos based on auth method
	loomURLs, err := scrapeVideos(config)
	if err != nil {
		log.Fatalf("Error scraping: %v", err)
	}

	if len(loomURLs) == 0 {
		fmt.Println(prefixError, "No videos found. Check authentication and URL.")
		return
	}

	fmt.Printf("%s Found %d video(s)\n", prefixSuccess, len(loomURLs))

	// Download each video
	for i, url := range loomURLs {
		fmt.Printf("\n[%d/%d] %s %s\n", i+1, len(loomURLs), prefixDownload, url)
		if err := downloadWithYtDlp(url, config.CookiesFile, config.OutputDir); err != nil {
			fmt.Printf("%s %v\n", prefixError, err)
		}
	}

	fmt.Println("\n" + prefixSuccess + " Download process completed!")
}

func printBanner() {
	fmt.Println(`
 ______     __  __     ______     ______     __            _____     __       
/\  ___\   /\ \/ /    /\  __ \   /\  __ \   /\ \          /\  __-.  /\ \      
\ \___  \  \ \  _"-.  \ \ \/\ \  \ \ \/\ \  \ \ \____     \ \ \/\ \ \ \ \____ 
 \/\_____\  \ \_\ \_\  \ \_____\  \ \_____\  \ \_____\     \ \____-  \ \_____\
  \/_____/   \/_/\/_/   \/_____/   \/_____/   \/_____/      \/____/   \/_____/
  		
  			Skool.com Video Downloader
		
			by Fx64b - github.com/fx64b
    `)
}

func parseFlags() Config {
	config := Config{}

	flag.StringVar(&config.SkoolURL, "url", "", "URL of the skool.com classroom to scrape (required)")
	flag.StringVar(&config.CookiesFile, "cookies", "", "Path to cookies file (JSON or TXT) for authentication")
	flag.StringVar(&config.Email, "email", "", "Email for Skool login (alternative to cookies)")
	flag.StringVar(&config.Password, "password", "", "Password for Skool login (required with email)")
	flag.StringVar(&config.OutputDir, "output", defaultOutputDir, "Directory to save downloaded videos")
	flag.IntVar(&config.WaitTime, "wait", defaultWaitTime, "Time to wait for page to load in seconds")
	flag.BoolVar(&config.Headless, "headless", defaultHeadless, "Run in headless mode (no browser UI)")
	flag.StringVar(&config.BrowserPath, "browser", "", "Path or command of browser to use (Chromium-based or Firefox, auto-detected if not specified)")

	flag.Parse()
	return config
}

func validateConfig(config Config) {
	if config.SkoolURL == "" {
		fmt.Println("Usage: skool-downloader -url=https://skool.com/yourschool/classroom/path [-cookies=cookies.json | -email=user@example.com -password=pass] [-browser=/path/to/browser]")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  -url        Skool classroom URL to scrape (required)")
		fmt.Println("  -email      Email address for Skool login")
		fmt.Println("  -password   Password for Skool login (required with -email)")
		fmt.Println("  -cookies    Path to cookies file (JSON or Netscape .txt)")
		fmt.Println("  -output     Directory to save downloaded videos (default: \"downloads\")")
		fmt.Println("  -wait       Seconds to wait for page load (default: 2)")
		fmt.Println("  -headless   Run browser in headless mode (default: true)")
		fmt.Println("  -browser    Path or command of browser to use (auto-detected if not set)")
		fmt.Println("              Supported: Edge, Chrome, Chromium, Brave, Arc, Firefox")
		fmt.Println("              Auto-detected in this order:")
		fmt.Println("                Windows : Edge > Chrome > Chromium > Brave > Firefox")
		fmt.Println("                macOS   : Chrome > Chromium > Edge > Brave > Arc > Firefox")
		fmt.Println("                Linux   : chromium-browser, chromium, google-chrome, ..., firefox")
		os.Exit(1)
	}

	usingEmail := config.Email != "" && config.Password != ""
	usingCookies := config.CookiesFile != ""

	if !usingEmail && !usingCookies {
		fmt.Println("Error: You must provide either cookies file or email+password for authentication")
		os.Exit(1)
	}
}

func scrapeVideos(config Config) ([]string, error) {
	if config.Email != "" && config.Password != "" {
		return scrapeWithLogin(config)
	}
	return scrapeWithCookies(config)
}

func getBrowserCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("PROGRAMFILES")
		programFilesX86 := os.Getenv("PROGRAMFILES(X86)")
		localAppData := os.Getenv("LOCALAPPDATA")

		if programFiles == "" {
			programFiles = `C:\Program Files`
		}
		if programFilesX86 == "" {
			programFilesX86 = `C:\Program Files (x86)`
		}
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}

		return []string{
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Chromium", "Application", "chrome.exe"),
			filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(programFilesX86, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(programFiles, "Mozilla Firefox", "firefox.exe"),
			filepath.Join(programFilesX86, "Mozilla Firefox", "firefox.exe"),
		}

	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Arc.app/Contents/MacOS/Arc",
			"/Applications/Firefox.app/Contents/MacOS/firefox",
		}

	default:
		return []string{
			"chromium-browser",
			"chromium",
			"google-chrome",
			"google-chrome-stable",
			"google-chrome-beta",
			"microsoft-edge",
			"microsoft-edge-stable",
			"brave-browser",
			"firefox",
			"firefox-esr",
		}
	}
}

func findBrowser(customPath string) (string, error) {
	if customPath != "" {
		if filepath.IsAbs(customPath) {
			if _, err := os.Stat(customPath); err == nil {
				return customPath, nil
			}
		} else {
			if path, err := exec.LookPath(customPath); err == nil {
				return path, nil
			}
		}
		return "", fmt.Errorf("specified browser not found: %s", customPath)
	}

	for _, candidate := range getBrowserCandidates() {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		} else {
			if path, err := exec.LookPath(candidate); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf(
		"no supported browser found.\n" +
			"Supported browsers: Microsoft Edge (built-in on Windows 10/11), Google Chrome, Chromium, Brave, Firefox.\n" +
			"Install one of the above, or specify an explicit path with: -browser=/path/to/browser",
	)
}

func isFirefox(path string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(path)), "firefox")
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitForFirefoxCDP(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		// Poll both addresses: Linux may resolve localhost to ::1 (IPv6) while
		// Firefox binds to 127.0.0.1 (IPv4), or vice versa.
		for _, host := range []string{"127.0.0.1", "localhost"} {
			resp, err := client.Get(fmt.Sprintf("http://%s:%d/json/version", host, port))
			if err == nil {
				var info struct {
					WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
				}
				if json.NewDecoder(resp.Body).Decode(&info) == nil && info.WebSocketDebuggerURL != "" {
					resp.Body.Close()
					return info.WebSocketDebuggerURL, nil
				}
				resp.Body.Close()
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for Firefox CDP on port %d", port)
}

func writeFirefoxPrefs(profileDir string) error {
	// These preferences are required for CDP remote debugging to work.
	// An empty profile would otherwise block on the first-run wizard and
	// show a "allow remote debugging?" prompt that prevents CDP from starting.
	prefs := `user_pref("devtools.debugger.remote-enabled", true);
user_pref("devtools.debugger.prompt-connection", false);
user_pref("devtools.chrome.enabled", true);
user_pref("browser.aboutwelcome.enabled", false);
user_pref("datareporting.policy.dataSubmissionEnabled", false);
user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
`
	return os.WriteFile(filepath.Join(profileDir, "prefs.js"), []byte(prefs), 0644)
}

// Firefox requires a different launch strategy: chromedp's ExecAllocator expects
// Chrome's startup output, so Firefox is started manually and connected via RemoteAllocator.
func setupFirefoxBrowser(headless bool, firefoxPath string) (context.Context, context.CancelFunc, error) {
	port, err := findFreePort()
	if err != nil {
		return nil, nil, fmt.Errorf("could not find free port: %w", err)
	}

	profileDir, err := os.MkdirTemp("", "skool-firefox-*")
	if err != nil {
		return nil, nil, fmt.Errorf("could not create temp profile: %w", err)
	}

	if err := writeFirefoxPrefs(profileDir); err != nil {
		os.RemoveAll(profileDir)
		return nil, nil, fmt.Errorf("could not write Firefox prefs: %w", err)
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-remote",
		"--profile", profileDir,
	}
	if headless {
		args = append(args, "--headless")
	}

	cmd := exec.Command(firefoxPath, args...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(profileDir)
		return nil, nil, fmt.Errorf("could not start Firefox: %w", err)
	}

	wsURL, err := waitForFirefoxCDP(port, 30*time.Second)
	if err != nil {
		cmd.Process.Kill()
		os.RemoveAll(profileDir)
		return nil, nil, fmt.Errorf("Firefox CDP not ready: %w", err)
	}

	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	ctx, cancel2 := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	ctx, cancel3 := context.WithTimeout(ctx, browserTimeout)

	return ctx, func() {
		cancel3()
		cancel2()
		cancel()
		cmd.Process.Kill()
		os.RemoveAll(profileDir)
	}, nil
}

func setupChromiumBrowser(headless bool, resolvedPath string) (context.Context, context.CancelFunc, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
		chromedp.ExecPath(resolvedPath),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel2 := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	ctx, cancel3 := context.WithTimeout(ctx, browserTimeout)

	return ctx, func() {
		cancel3()
		cancel2()
		cancel()
	}, nil
}

func setupBrowser(headless bool, browserPath string) (context.Context, context.CancelFunc, error) {
	resolvedPath, err := findBrowser(browserPath)
	if err != nil {
		return nil, nil, err
	}

	fmt.Printf("%s Using browser: %s\n", prefixInfo, resolvedPath)

	if isFirefox(resolvedPath) {
		return setupFirefoxBrowser(headless, resolvedPath)
	}
	return setupChromiumBrowser(headless, resolvedPath)
}

// extractNextDataJSON extracts the __NEXT_DATA__ JSON object from Skool's HTML
// This contains the complete course structure with all video URLs
func extractNextDataJSON(html string) (map[string]interface{}, error) {
	// Find the __NEXT_DATA__ script tag
	re := regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">([\s\S]*?)</script>`)
	matches := re.FindStringSubmatch(html)

	if len(matches) < 2 {
		return nil, fmt.Errorf("__NEXT_DATA__ script tag not found in HTML")
	}

	// Parse JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(matches[1]), &data); err != nil {
		return nil, fmt.Errorf("failed to parse __NEXT_DATA__ JSON: %w", err)
	}

	return data, nil
}

// extractLoomURLsFromNextData recursively walks the course structure in __NEXT_DATA__
// and extracts all video URLs (Loom and YouTube)
func extractLoomURLsFromNextData(data map[string]interface{}) []string {
	uniqueURLs := make(map[string]bool)
	var result []string

	// Navigate to course structure: data.props.pageProps.course
	props, ok := data["props"].(map[string]interface{})
	if !ok {
		return result
	}

	pageProps, ok := props["pageProps"].(map[string]interface{})
	if !ok {
		return result
	}

	course, ok := pageProps["course"].(map[string]interface{})
	if !ok {
		return result
	}

	// Recursive function to walk the course tree
	var walkCourseTree func(node map[string]interface{})
	walkCourseTree = func(node map[string]interface{}) {
		if node == nil {
			return
		}

		// Check if this node has course metadata with a videoLink
		if courseObj, ok := node["course"].(map[string]interface{}); ok {
			if metadata, ok := courseObj["metadata"].(map[string]interface{}); ok {
				if videoLink, ok := metadata["videoLink"].(string); ok {
					// Check if it's a Loom URL
					if strings.Contains(videoLink, "loom.com") {
						// Extract video ID from URL
						loomIDRegex := regexp.MustCompile(`loom\.com/(share|embed)/([a-zA-Z0-9_-]+)`)
						if matches := loomIDRegex.FindStringSubmatch(videoLink); len(matches) >= 3 {
							videoID := matches[2]
							// Normalize to share URL format
							shareURL := fmt.Sprintf("https://www.loom.com/share/%s", videoID)
							if !uniqueURLs[shareURL] {
								uniqueURLs[shareURL] = true
								result = append(result, shareURL)
							}
						}
					} else if strings.Contains(videoLink, "youtube.com") || strings.Contains(videoLink, "youtu.be") {
						// Extract and normalize YouTube URL
						normalizedURL := normalizeYouTubeURL(videoLink)
						if normalizedURL != "" && !uniqueURLs[normalizedURL] {
							uniqueURLs[normalizedURL] = true
							result = append(result, normalizedURL)
						}
					}
				}
			}
		}

		// Recursively process children (sets and modules)
		if children, ok := node["children"].([]interface{}); ok {
			for _, child := range children {
				if childMap, ok := child.(map[string]interface{}); ok {
					walkCourseTree(childMap)
				}
			}
		}
	}

	// Start walking from the course root
	walkCourseTree(course)

	return result
}

// normalizeYouTubeURL extracts video ID and normalizes YouTube URL to standard watch format
func normalizeYouTubeURL(videoLink string) string {
	// Regex patterns for different YouTube URL formats
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/)([a-zA-Z0-9_-]{11})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(videoLink); len(matches) >= 2 {
			videoID := matches[1]
			return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
		}
	}

	return ""
}

// extractLoomURLs extracts video URLs (Loom and YouTube) from HTML
// NEW APPROACH: Try __NEXT_DATA__ JSON first (fast, accurate), fallback to regex (old method)
func extractLoomURLs(html string) []string {
	// Try extracting from __NEXT_DATA__ JSON first
	if nextData, err := extractNextDataJSON(html); err == nil {
		urls := extractLoomURLsFromNextData(nextData)
		if len(urls) > 0 {
			fmt.Printf("%s Extracted %d video(s) from __NEXT_DATA__ JSON\n", prefixInfo, len(urls))
			return urls
		}
		fmt.Println(prefixWarning, "No videos found in __NEXT_DATA__, falling back to regex extraction")
	} else {
		fmt.Printf("%s __NEXT_DATA__ extraction failed (%v), falling back to regex extraction\n", prefixWarning, err)
	}

	// Fallback to old regex-based extraction
	// Loom patterns
	loomShareRegex := regexp.MustCompile(`https?://(?:www\.)?loom\.com/share/[a-zA-Z0-9]+`)
	loomEmbedRegex := regexp.MustCompile(`https?://(?:www\.)?loom\.com/embed/([a-zA-Z0-9]+)`)

	// YouTube patterns
	youtubeRegex := regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/)([a-zA-Z0-9_-]{11})`)

	var matches []string

	// Extract Loom share URLs
	matches = append(matches, loomShareRegex.FindAllString(html, -1)...)

	// Convert Loom embed URLs to share URLs
	loomEmbedMatches := loomEmbedRegex.FindAllStringSubmatch(html, -1)
	for _, match := range loomEmbedMatches {
		if len(match) >= 2 {
			shareURL := fmt.Sprintf("https://www.loom.com/share/%s", match[1])
			matches = append(matches, shareURL)
		}
	}

	// Extract and normalize YouTube URLs
	youtubeMatches := youtubeRegex.FindAllStringSubmatch(html, -1)
	for _, match := range youtubeMatches {
		if len(match) >= 2 {
			videoID := match[1]
			watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
			matches = append(matches, watchURL)
		}
	}

	// Remove duplicates
	uniqueURLs := make(map[string]bool)
	var result []string
	for _, url := range matches {
		if !uniqueURLs[url] {
			uniqueURLs[url] = true
			result = append(result, url)
		}
	}

	if len(result) > 0 {
		fmt.Printf("%s Extracted %d video(s) from regex patterns\n", prefixInfo, len(result))
	}

	return result
}

func scrapeWithLogin(config Config) ([]string, error) {
	ctx, cancel, err := setupBrowser(config.Headless, config.BrowserPath)
	if err != nil {
		return nil, err
	}
	defer cancel()

	var currentURL string
	var loginSuccess bool

	fmt.Println(prefixAuth, "Attempting login with email and password...")

	// Navigate to the main Skool site
	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(skoolBaseURL),
		chromedp.Sleep(initialWaitTime),
		chromedp.Location(&currentURL),
	}); err != nil {
		return nil, fmt.Errorf("failed to navigate to Skool: %v", err)
	}

	fmt.Println(prefixInfo, "Landed on:", currentURL)

	// Try to find and click the login button
	err = chromedp.Run(ctx, chromedp.Tasks{
		chromedp.WaitVisible(`//button[@type="button"]/span[text()="Log In"]`, chromedp.BySearch),
		chromedp.Click(`//button[@type="button"]/span[text()="Log In"]`, chromedp.BySearch),
		chromedp.Sleep(2 * time.Second),
		chromedp.Location(&currentURL),
	})

	// If login button not found, navigate directly to login page
	if err != nil {
		fmt.Println(prefixWarning, "Couldn't find login button, trying direct navigation to login page...")
		if err := chromedp.Run(ctx, chromedp.Tasks{
			chromedp.Navigate(skoolLoginURL),
			chromedp.Sleep(initialWaitTime),
			chromedp.Location(&currentURL),
		}); err != nil {
			return nil, fmt.Errorf("couldn't access login page: %v", err)
		}
	}

	fmt.Println(prefixInfo, "Login page:", currentURL)

	// Complete the login form
	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.WaitVisible(`//input[@type="email" or @name="email" or contains(@placeholder, "email")]`, chromedp.BySearch),
		chromedp.SendKeys(`//input[@type="email" or @name="email" or contains(@placeholder, "email")]`, config.Email, chromedp.BySearch),

		chromedp.WaitVisible(`//input[@type="password" or @name="password" or contains(@placeholder, "password")]`, chromedp.BySearch),
		chromedp.SendKeys(`//input[@type="password" or @name="password" or contains(@placeholder, "password")]`, config.Password, chromedp.BySearch),

		chromedp.Click(`//button[@type="submit" and .//span[contains(text(), "Log") or contains(text(), "Log In") or contains(text(), "Login")]]`, chromedp.BySearch),

		chromedp.Sleep(loginWaitTime),
		chromedp.Location(&currentURL),
		chromedp.Evaluate(`!window.location.href.includes('/login') && !document.body.textContent.includes('Incorrect password') && !document.body.textContent.includes('No account found for this email.')`, &loginSuccess),
	}); err != nil {
		return nil, fmt.Errorf("login process failed: %v", err)
	}

	if !loginSuccess {
		return nil, fmt.Errorf("login failed: invalid credentials or captcha required")
	}

	fmt.Println(prefixSuccess, "Login successful! Redirected to:", currentURL)
	return navigateAndScrape(ctx, config.SkoolURL, config.WaitTime)
}

func scrapeWithCookies(config Config) ([]string, error) {
	ctx, cancel, err := setupBrowser(config.Headless, config.BrowserPath)
	if err != nil {
		return nil, err
	}
	defer cancel()

	// Load and set cookies
	cookies, err := parseCookiesFile(config.CookiesFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing cookies: %v", err)
	}

	// Log cookie info
	fmt.Println(prefixAuth, "Setting cookies...")
	for _, c := range cookies {
		if c.Name == "auth_token" && strings.Contains(c.Domain, "skool") {
			truncatedValue := c.Value
			if len(truncatedValue) > 20 {
				truncatedValue = truncatedValue[:20] + "..."
			}
			fmt.Printf("%s Auth token found: %s\n", prefixAuth, truncatedValue)
		}
	}

	// Enable network and set cookies
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		return nil, err
	}

	if err := chromedp.Run(ctx, network.SetCookies(cookies)); err != nil {
		return nil, fmt.Errorf("error setting cookies: %v", err)
	}

	var currentURL string
	// Set headers and navigate first to main site, then to target URL
	err = chromedp.Run(ctx, chromedp.Tasks{
		network.SetExtraHTTPHeaders(network.Headers{
			"Referer":         skoolBaseURL,
			"Accept":          "text/html,application/xhtml+xml,application/xml",
			"Accept-Language": "en-US,en;q=0.9",
			"Connection":      "keep-alive",
		}),
		chromedp.Navigate(skoolBaseURL),
		chromedp.Sleep(initialWaitTime),
		chromedp.Location(&currentURL),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to navigate to main site: %v", err)
	}

	fmt.Printf("%s Initial navigation landed on: %s\n", prefixInfo, currentURL)
	return navigateAndScrape(ctx, config.SkoolURL, config.WaitTime)
}

func navigateAndScrape(ctx context.Context, targetURL string, waitTime int) ([]string, error) {
	var currentURL, html string

	fmt.Println(prefixInfo, "Navigating to classroom:", targetURL)
	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(targetURL),
		chromedp.Sleep(time.Duration(waitTime) * time.Second),
		chromedp.Location(&currentURL),
	}); err != nil {
		return nil, fmt.Errorf("failed to navigate to classroom: %v", err)
	}

	fmt.Println(prefixInfo, "Landed on:", currentURL)

	// Check if we're on the right page
	if strings.Contains(currentURL, "/about") {
		return nil, fmt.Errorf("authentication succeeded but redirected to public page, check URL permissions")
	}

	// Get page content
	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.OuterHTML("html", &html),
	}); err != nil {
		return nil, err
	}

	// Extract and return video URLs
	urls := extractLoomURLs(html)
	if len(urls) == 0 {
		fmt.Println(prefixWarning, "No videos found on the page.")
	}

	return urls, nil
}

// Cookie parsing functions
func parseCookiesFile(filePath string) ([]*network.CookieParam, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Determine file type based on extension and content
	isJSON := strings.HasSuffix(strings.ToLower(filePath), ".json")
	if !isJSON && !strings.HasSuffix(strings.ToLower(filePath), ".txt") {
		trimmed := strings.TrimSpace(string(content))
		isJSON = strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
	}

	if isJSON {
		return parseJSONCookies(content)
	}
	return parseNetscapeCookies(content)
}

func parseJSONCookies(content []byte) ([]*network.CookieParam, error) {
	var jsonCookies []JSONCookie
	if err := json.Unmarshal(content, &jsonCookies); err != nil {
		return nil, fmt.Errorf("error parsing JSON cookies: %v", err)
	}

	var cookies []*network.CookieParam
	for _, c := range jsonCookies {
		// Clean up the host field (remove leading dot if present)
		domain := strings.TrimPrefix(c.Host, ".")

		cookie := &network.CookieParam{
			Domain:   domain,
			Name:     c.Name,
			Value:    c.Value,
			Path:     c.Path,
			Secure:   c.IsSecure == 1,
			HTTPOnly: c.IsHttpOnly == 1,
		}

		// Convert SameSite value
		switch c.SameSite {
		case 1:
			cookie.SameSite = network.CookieSameSiteLax
		case 2:
			cookie.SameSite = network.CookieSameSiteStrict
		case 3:
			cookie.SameSite = network.CookieSameSiteNone
		}

		// Add expiry if present
		if c.Expiry > 0 {
			t := cdp.TimeSinceEpoch(time.Unix(c.Expiry, 0))
			cookie.Expires = &t
		}

		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

func parseNetscapeCookies(content []byte) ([]*network.CookieParam, error) {
	lines := strings.Split(string(content), "\n")
	var cookies []*network.CookieParam

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		domain := strings.TrimPrefix(fields[0], ".")

		cookie := &network.CookieParam{
			Domain:   domain,
			Path:     fields[2],
			Secure:   fields[3] == "TRUE",
			Name:     fields[5],
			Value:    fields[6],
			HTTPOnly: false,
		}

		// Try to parse expiry if present
		if len(fields) > 4 {
			expiryStr := fields[4]
			if expiryStr != "" && expiryStr != "0" {
				expiry, err := parseInt64(expiryStr)
				if err == nil && expiry > 0 {
					t := cdp.TimeSinceEpoch(time.Unix(expiry, 0))
					cookie.Expires = &t
				}
			}
		}

		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func downloadWithYtDlp(videoURL, cookiesFile, outputDir string) error {
	args := []string{
		"-o", filepath.Join(outputDir, "%(title)s.%(ext)s"),
		"--no-warnings",
		videoURL,
	}

	// Only add cookies argument if a cookies file is provided
	if cookiesFile != "" {
		tmpCookiesFile := cookiesFile
		isJSON := strings.HasSuffix(strings.ToLower(cookiesFile), ".json")

		if isJSON {
			tmpFile, err := convertJSONToNetscapeCookies(cookiesFile)
			if err != nil {
				return fmt.Errorf("error converting JSON cookies: %v", err)
			}
			defer func() {
				_ = os.Remove(tmpFile)
			}()
			tmpCookiesFile = tmpFile
		}

		// Add cookies argument only when we have a valid file
		args = append([]string{"--cookies", tmpCookiesFile}, args...)
	}

	cmd := exec.Command("yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func convertJSONToNetscapeCookies(jsonFile string) (string, error) {
	content, err := os.ReadFile(jsonFile)
	if err != nil {
		return "", err
	}

	var jsonCookies []JSONCookie
	if err := json.Unmarshal(content, &jsonCookies); err != nil {
		return "", err
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "cookies-*.txt")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
	}()

	// Write header
	fmt.Fprintln(tmpFile, "# Netscape HTTP Cookie File")
	fmt.Fprintln(tmpFile, "# This file was generated by skool-downloader")

	// Write cookies
	for _, c := range jsonCookies {
		host := c.Host
		if !strings.HasPrefix(host, ".") && strings.Count(host, ".") > 1 {
			host = "." + host
		}

		secure := "FALSE"
		if c.IsSecure == 1 {
			secure = "TRUE"
		}

		// Format: DOMAIN FLAG PATH SECURE EXPIRY NAME VALUE
		if _, err := fmt.Fprintf(tmpFile, "%s\tTRUE\t%s\t%s\t%d\t%s\t%s\n",
			host, c.Path, secure, c.Expiry, c.Name, c.Value); err != nil {
			return "", err
		}
	}

	return tmpFile.Name(), nil
}
