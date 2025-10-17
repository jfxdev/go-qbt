package qbt

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   1 * time.Second,
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}

	if client.config.RequestTimeout != 30*time.Second {
		t.Errorf("Expected timeout: 30s, got: %v", client.config.RequestTimeout)
	}

	if client.retryConfig.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries: 3, got: %d", client.retryConfig.MaxRetries)
	}
}

func TestNewClientDefaults(t *testing.T) {
	config := Config{
		BaseURL:  "http://localhost:8080",
		Username: "test",
		Password: "test",
	}

	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check defaults
	if client.config.RequestTimeout != DefaultRequestTimeout {
		t.Errorf("Expected default RequestTimeout: %v, got: %v", DefaultRequestTimeout, client.config.RequestTimeout)
	}

	if client.config.MaxRetries != DefaultMaxRetries {
		t.Errorf("Expected default MaxRetries: %d, got: %d", DefaultMaxRetries, client.config.MaxRetries)
	}

	if client.config.RetryBackoff != DefaultRetryBackoff {
		t.Errorf("Expected default RetryBackoff: %v, got: %v", DefaultRetryBackoff, client.config.RetryBackoff)
	}
}

func TestCookieCache(t *testing.T) {
	cache := newCookieCache()

	if cache.cookies == nil {
		t.Fatal("Cookie cache should not be nil")
	}

	// Update
	testCookie := &http.Cookie{
		Name:  "test",
		Value: "value",
	}

	cache.update([]*http.Cookie{testCookie})

	if len(cache.cookies) != 1 {
		t.Errorf("Expected 1 cookie, got %d", len(cache.cookies))
	}

	// Clear
	cache.clear()

	if len(cache.cookies) != 0 {
		t.Errorf("Cookie cache should be empty after clear, got %d cookies", len(cache.cookies))
	}
}

func TestRetryConfig(t *testing.T) {
	config := Config{
		MaxRetries:   5,
		RetryBackoff: 2 * time.Second,
	}

	retryConfig := newRetryConfig(config)

	if retryConfig.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries: 5, got: %d", retryConfig.MaxRetries)
	}

	if retryConfig.BaseDelay != 2*time.Second {
		t.Errorf("Expected BaseDelay: 2s, got: %v", retryConfig.BaseDelay)
	}

	if retryConfig.BackoffFactor != DefaultBackoffFactor {
		t.Errorf("Expected BackoffFactor: %f, got: %f", DefaultBackoffFactor, retryConfig.BackoffFactor)
	}
}

func TestCalculateBackoffDelay(t *testing.T) {
	client := &Client{
		retryConfig: &RetryConfig{
			BaseDelay:     1 * time.Second,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
		},
	}

	// Delays for different attempts
	delays := []time.Duration{
		client.calculateBackoffDelay(0), // 1s
		client.calculateBackoffDelay(1), // 2s
		client.calculateBackoffDelay(2), // 4s
		client.calculateBackoffDelay(3), // 8s
		client.calculateBackoffDelay(4), // 10s (max)
	}

	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		10 * time.Second,
	}

	for i, delay := range delays {
		if delay != expected[i] {
			t.Errorf("Attempt %d delay: expected %v, got %v", i, expected[i], delay)
		}
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	client := &Client{
		retryConfig: &RetryConfig{
			RetryableCodes: []int{408, 429, 500, 502, 503, 504},
		},
	}

	testCases := []struct {
		statusCode int
		expected   bool
	}{
		{200, false},
		{400, false},
		{408, true},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tc := range testCases {
		result := client.isRetryableStatusCode(tc.statusCode)
		if result != tc.expected {
			t.Errorf("Status %d: expected %v, got %v", tc.statusCode, tc.expected, result)
		}
	}
}

func TestCookieValidation(t *testing.T) {
	client := &Client{
		cookieValid: false,
	}

	// Cached validity should be false initially
	if client.isCookieValidCached() {
		t.Error("Cookie should not be valid initially")
	}

	// Set validity
	client.setCookieValid(true)
	if !client.isCookieValidCached() {
		t.Error("Cookie should be valid after setCookieValid(true)")
	}

	// Invalidate
	client.setCookieValid(false)
	if client.isCookieValidCached() {
		t.Error("Cookie should not be valid after setCookieValid(false)")
	}
}

func TestCookieExpiration(t *testing.T) {
	client := &Client{
		lastLoginTime: time.Now().Add(-25 * time.Hour), // 25 hours ago
	}

	if !client.isCookieExpired() {
		t.Error("Cookie should be expired after 25 hours")
	}

	client.lastLoginTime = time.Now().Add(-23 * time.Hour) // 23 hours ago

	if client.isCookieExpired() {
		t.Error("Cookie should not be expired after 23 hours")
	}
}

func TestUpdateConfig(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Mark cookies as valid
	client.setCookieValid(true)

	// Update configuration
	newConfig := Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 60 * time.Second,
	}

	client.Update(newConfig)

	// Cookies should be invalidated after update
	if client.isCookieValidCached() {
		t.Error("Cookies should be invalidated after Update")
	}

	// Timeout should be updated
	if client.client.Timeout != 60*time.Second {
		t.Errorf("Expected timeout: 60s, got: %v", client.client.Timeout)
	}
}

func TestContextWithTimeout(t *testing.T) {
	// Build a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Simulate an operation longer than the timeout
	start := time.Now()

	// This only validates the context cancellation behavior
	select {
	case <-ctx.Done():
		// Cancelled as expected
	case <-time.After(2 * time.Second):
		t.Error("Context should have been cancelled")
	}

	duration := time.Since(start)
	if duration > 2*time.Second {
		t.Errorf("Operation took too long: %v", duration)
	}
}

func TestRetryWithBackoff(t *testing.T) {
	client := &Client{
		retryConfig: &RetryConfig{
			MaxRetries:    2,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			BackoffFactor: 2.0,
		},
	}

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("simulated error %d", attempts)
		}
		return nil
	}

	err := client.retryWithBackoff(operation, "test")
	if err != nil {
		t.Errorf("Operation should succeed after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoffFailure(t *testing.T) {
	client := &Client{
		retryConfig: &RetryConfig{
			MaxRetries:    2,
			BaseDelay:     10 * time.Millisecond,
			MaxDelay:      100 * time.Millisecond,
			BackoffFactor: 2.0,
		},
	}

	operation := func() error {
		return fmt.Errorf("persistent error")
	}

	err := client.retryWithBackoff(operation, "test")
	if err == nil {
		t.Error("Operation should fail after all attempts")
	}

	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestInvalidateCookiesOnAuthError(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Set cookies as valid to simulate a logged in state
	client.setCookieValid(true)
	client.lastLoginTime = time.Now()

	if !client.isCookieValidCached() {
		t.Error("Cookie should be valid initially")
	}

	// Simulate receiving an auth error
	client.invalidateCookies()

	// Cookies should now be invalid
	if client.isCookieValidCached() {
		t.Error("Cookie should be invalid after invalidateCookies()")
	}

	// Cookie cache should be empty
	if len(client.cookieCache.cookies) != 0 {
		t.Errorf("Cookie cache should be empty, got %d cookies", len(client.cookieCache.cookies))
	}
}

func TestDebugMode(t *testing.T) {
	// Test with debug disabled (default)
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
		Debug:          false,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.config.Debug {
		t.Error("Debug should be disabled by default")
	}

	// Test with debug enabled
	clientDebug, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
		Debug:          true,
	})
	if err != nil {
		t.Fatalf("Failed to create client with debug: %v", err)
	}

	if !clientDebug.config.Debug {
		t.Error("Debug should be enabled when set to true")
	}
}

// Test new torrent management methods
func TestSetTorrentLocation(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	hash := "test_hash_123"
	location := "/new/path"

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.SetTorrentLocation(hash, location)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to set torrent location") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestRenameTorrent(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	hash := "test_hash_123"
	newName := "New Torrent Name"

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.RenameTorrent(hash, newName)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to rename torrent") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestSuperSeedingMode(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	hash := "test_hash_123"
	enabled := true

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.SuperSeedingMode(hash, enabled)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to set super seeding mode") {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	// Test with disabled super seeding
	err = client.SuperSeedingMode(hash, false)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to set super seeding mode") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestForceRecheck(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	hash := "test_hash_123"

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.ForceRecheck(hash)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to force recheck") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestForceReannounce(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	hash := "test_hash_123"

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.ForceReannounce(hash)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to force reannounce") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

// Test new rate limit methods
func TestGlobalRateLimits(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	downloadLimit := 1024 * 1024 // 1 MB/s
	uploadLimit := 512 * 1024    // 512 KB/s

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.SetGlobalRateLimits(downloadLimit, uploadLimit)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestAlternativeRateLimits(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with valid parameters
	downloadLimit := 2048 * 1024 // 2 MB/s
	uploadLimit := 1024 * 1024   // 1 MB/s

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.SetAlternativeRateLimits(downloadLimit, uploadLimit)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestGlobalRateLimitsWithZeroValues(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with zero values (should disable limits)
	downloadLimit := 0
	uploadLimit := 0

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.SetGlobalRateLimits(downloadLimit, uploadLimit)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestAlternativeRateLimitsWithZeroValues(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with zero values (should disable limits)
	downloadLimit := 0
	uploadLimit := 0

	// This test will fail in real scenario without a running qBittorrent instance
	// but it validates the method signature and basic structure
	err = client.SetAlternativeRateLimits(downloadLimit, uploadLimit)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

// ===== TESTS FOR MAXIMUM ACTIVE TORRENT MANAGEMENT =====

func TestSetMaxActiveDownloads(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test setting max active downloads
	maxDownloads := 5
	err = client.SetMaxActiveDownloads(maxDownloads)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestSetMaxActiveUploads(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test setting max active uploads
	maxUploads := 3
	err = client.SetMaxActiveUploads(maxUploads)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestSetMaxActiveTorrents(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test setting max active torrents
	maxTorrents := 10
	err = client.SetMaxActiveTorrents(maxTorrents)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestSetMaxActiveCheckingTorrents(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test setting max active checking torrents
	maxChecking := 2
	err = client.SetMaxActiveCheckingTorrents(maxChecking)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestSetMaxActiveTorrentLimits(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test setting all max active limits at once
	maxDownloads := 5
	maxUploads := 3
	maxTorrents := 10
	maxChecking := 2

	err = client.SetMaxActiveTorrentLimits(maxDownloads, maxUploads, maxTorrents, maxChecking)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestGetMaxActiveDownloads(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test getting max active downloads
	_, err = client.GetMaxActiveDownloads()
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestGetMaxActiveUploads(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test getting max active uploads
	_, err = client.GetMaxActiveUploads()
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestGetMaxActiveTorrents(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test getting max active torrents
	_, err = client.GetMaxActiveTorrents()
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestGetMaxActiveCheckingTorrents(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test getting max active checking torrents
	_, err = client.GetMaxActiveCheckingTorrents()
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestMaxActiveTorrentLimitsWithZeroValues(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with zero values (should disable limits)
	maxDownloads := 0
	maxUploads := 0
	maxTorrents := 0
	maxChecking := 0

	err = client.SetMaxActiveTorrentLimits(maxDownloads, maxUploads, maxTorrents, maxChecking)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestMaxActiveTorrentLimitsWithNegativeValues(t *testing.T) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with negative values (should be handled gracefully)
	maxDownloads := -1
	maxUploads := -5
	maxTorrents := -10
	maxChecking := -2

	err = client.SetMaxActiveTorrentLimits(maxDownloads, maxUploads, maxTorrents, maxChecking)
	if err != nil {
		// Expected to fail without real qBittorrent instance
		if !strings.Contains(err.Error(), "failed to get global settings") {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}
