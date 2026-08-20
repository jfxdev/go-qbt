package qbt

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestSetTorrentLocation(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SetTorrentLocation("test_hash_123", "/new/path"); err != nil {
		t.Fatalf("SetTorrentLocation returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/setLocation",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes":   "test_hash_123",
				"location": "/new/path",
			},
		},
	})
}

func TestRenameTorrent(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.RenameTorrent("test_hash_123", "New Torrent Name"); err != nil {
		t.Fatalf("RenameTorrent returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/rename",
			Method: http.MethodPost,
			Form: map[string]string{
				"hash": "test_hash_123",
				"name": "New Torrent Name",
			},
		},
	})
}

func TestGetTags(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/tags"] = endpointResponse{
		body: `["movies","4k","genre::action"]`,
	}
	client := newTestClient(t, recorder.handler())

	tags, err := client.GetTags()
	if err != nil {
		t.Fatalf("GetTags returned error: %v", err)
	}

	want := []string{"movies", "4k", "genre::action"}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %d (%v)", len(want), len(tags), tags)
	}
	for i, tag := range want {
		if tags[i] != tag {
			t.Fatalf("tag %d: expected %s, got %s", i, tag, tags[i])
		}
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{Path: "/api/v2/torrents/tags", Method: http.MethodGet},
	})
}

func TestGetTags_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/tags"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "boom",
	}
	client := newTestClient(t, recorder.handler())

	_, err := client.GetTags()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get tags") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestCreateTags(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.CreateTags([]string{"movies", "4k"}); err != nil {
		t.Fatalf("CreateTags returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/createTags",
			Method: http.MethodPost,
			Form: map[string]string{
				"tags": "movies,4k",
			},
		},
	})
}

func TestCreateTags_EmptyIsNoop(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.CreateTags(nil); err != nil {
		t.Fatalf("CreateTags returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{})
}

func TestCreateTags_RejectsCommaInName(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.CreateTags([]string{"movies,4k"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "movies,4k") {
		t.Fatalf("expected error to name the offending tag, got: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{})
}

func TestCreateTags_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/createTags"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "invalid tag",
	}
	client := newTestClient(t, recorder.handler())

	err := client.CreateTags([]string{"movies"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create tags") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid tag") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestDeleteTags(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.DeleteTags([]string{"movies", "4k"}); err != nil {
		t.Fatalf("DeleteTags returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/deleteTags",
			Method: http.MethodPost,
			Form: map[string]string{
				"tags": "movies,4k",
			},
		},
	})
}

func TestDeleteTags_EmptyIsNoop(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.DeleteTags(nil); err != nil {
		t.Fatalf("DeleteTags returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{})
}

func TestDeleteTags_RejectsCommaInName(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.DeleteTags([]string{"movies,4k"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "movies,4k") {
		t.Fatalf("expected error to name the offending tag, got: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{})
}

func TestDeleteTags_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/deleteTags"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "boom",
	}
	client := newTestClient(t, recorder.handler())

	err := client.DeleteTags([]string{"movies"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to delete tags") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestSuperSeedingMode(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SuperSeedingMode("test_hash_123", true); err != nil {
		t.Fatalf("SuperSeedingMode(true) returned error: %v", err)
	}
	if err := client.SuperSeedingMode("test_hash_123", false); err != nil {
		t.Fatalf("SuperSeedingMode(false) returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/setSuperSeeding",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes": "test_hash_123",
				"value":  "true",
			},
		},
		{
			Path:   "/api/v2/torrents/setSuperSeeding",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes": "test_hash_123",
				"value":  "false",
			},
		},
	})
}

func TestForceRecheck(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.ForceRecheck("test_hash_123"); err != nil {
		t.Fatalf("ForceRecheck returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/recheck",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes": "test_hash_123",
			},
		},
	})
}

func TestForceReannounce(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.ForceReannounce("test_hash_123"); err != nil {
		t.Fatalf("ForceReannounce returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/reannounce",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes": "test_hash_123",
			},
		},
	})
}

func TestAlternativeRateLimits(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SetAlternativeRateLimits(2048*1024, 1024*1024); err != nil {
		t.Fatalf("SetAlternativeRateLimits returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/app/setPreferences",
			Method: http.MethodPost,
			JSONField: map[string]interface{}{
				"alt_dl_limit": float64(2048 * 1024),
				"alt_up_limit": float64(1024 * 1024),
			},
		},
	})
}

func TestAlternativeRateLimitsWithZeroValues(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SetAlternativeRateLimits(0, 0); err != nil {
		t.Fatalf("SetAlternativeRateLimits with zero values returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/app/setPreferences",
			Method: http.MethodPost,
			JSONField: map[string]interface{}{
				"alt_dl_limit": float64(0),
				"alt_up_limit": float64(0),
			},
		},
	})
}

func TestSetAlternativeRateLimits_PropagatesSetPreferencesError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/app/setPreferences"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "invalid rate limits",
	}
	client := newTestClient(t, recorder.handler())

	err := client.SetAlternativeRateLimits(1, 2)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to set alternative rate limits") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid rate limits") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestMaxActiveTorrentLimitsWithZeroValues(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SetMaxActiveTorrentLimits(0, 0, 0, -1); err != nil {
		t.Fatalf("SetMaxActiveTorrentLimits returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/app/setPreferences",
			Method: http.MethodPost,
			JSONField: map[string]interface{}{
				"max_active_downloads":         float64(0),
				"max_active_uploads":           float64(0),
				"max_active_torrents":          float64(0),
				"max_active_checking_torrents": float64(-1),
			},
		},
	})
}

func TestMaxActiveTorrentLimitsWithNegativeValues(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SetMaxActiveTorrentLimits(-1, -5, -10, -1); err != nil {
		t.Fatalf("SetMaxActiveTorrentLimits with negative values returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/app/setPreferences",
			Method: http.MethodPost,
			JSONField: map[string]interface{}{
				"max_active_downloads":         float64(-1),
				"max_active_uploads":           float64(-5),
				"max_active_torrents":          float64(-10),
				"max_active_checking_torrents": float64(-1),
			},
		},
	})
}

func TestDoWithRetryUsesConfiguredRequestTimeout(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/setLocation"] = endpointResponse{
		delay:      150 * time.Millisecond,
		statusCode: http.StatusOK,
	}
	client := newTestClient(t, recorder.handler())
	client.config.RequestTimeout = 50 * time.Millisecond
	client.client.Timeout = 50 * time.Millisecond
	client.retryConfig.MaxRetries = 0

	start := time.Now()
	err := client.SetTorrentLocation("test_hash_123", "/slow/path")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed > 130*time.Millisecond {
		t.Fatalf("expected request timeout to stop early, took %v", elapsed)
	}
}

type expectedCall struct {
	Path      string
	Method    string
	Form      map[string]string
	JSONField map[string]interface{}
}

type endpointResponse struct {
	statusCode int
	body       string
	delay      time.Duration
	headers    map[string]string
}

type recordedCall struct {
	Path   string
	Method string
	Form   url.Values
}

type apiTestRecorder struct {
	t         *testing.T
	calls     []recordedCall
	responses map[string]endpointResponse
}

func newAPITestRecorder(t *testing.T) *apiTestRecorder {
	return &apiTestRecorder{
		t:         t,
		responses: make(map[string]endpointResponse),
	}
}

func (r *apiTestRecorder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			r.t.Fatalf("failed to parse form for %s: %v", req.URL.Path, err)
		}

		r.calls = append(r.calls, recordedCall{
			Path:   req.URL.Path,
			Method: req.Method,
			Form:   req.PostForm,
		})

		switch req.URL.Path {
		case "/api/v2/app/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("4.6.0"))
			return
		case "/api/v2/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "cookie"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Ok."))
			return
		}

		resp, ok := r.responses[req.URL.Path]
		if !ok {
			resp = endpointResponse{statusCode: http.StatusOK}
		}
		if resp.delay > 0 {
			time.Sleep(resp.delay)
		}
		for key, value := range resp.headers {
			w.Header().Set(key, value)
		}
		if resp.statusCode == 0 {
			resp.statusCode = http.StatusOK
		}
		w.WriteHeader(resp.statusCode)
		if resp.body != "" {
			_, _ = w.Write([]byte(resp.body))
		}
	})
}

func (r *apiTestRecorder) assertCalls(t *testing.T, expected []expectedCall) {
	t.Helper()

	if len(r.calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(r.calls))
	}

	for i, want := range expected {
		got := r.calls[i]
		if got.Path != want.Path {
			t.Fatalf("call %d path: expected %s, got %s", i, want.Path, got.Path)
		}
		if got.Method != want.Method {
			t.Fatalf("call %d method: expected %s, got %s", i, want.Method, got.Method)
		}
		for key, value := range want.Form {
			if got.Form.Get(key) != value {
				t.Fatalf("call %d form %s: expected %s, got %s", i, key, value, got.Form.Get(key))
			}
		}
		if want.JSONField != nil {
			raw := got.Form.Get("json")
			if raw == "" {
				t.Fatalf("call %d expected json field, got empty form", i)
			}
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				t.Fatalf("call %d invalid json payload: %v", i, err)
			}
			for key, value := range want.JSONField {
				if decoded[key] != value {
					t.Fatalf("call %d json %s: expected %v, got %v", i, key, value, decoded[key])
				}
			}
		}
	}
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	server := newIPv4TestServer(t, handler)
	t.Cleanup(server.Close)

	client, err := New(Config{
		BaseURL:        server.URL,
		Username:       "test",
		Password:       "test",
		RequestTimeout: 200 * time.Millisecond,
		MaxRetries:     0,
		RetryBackoff:   5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Cleanup(func() {
		client.invalidateCookies()
	})

	return client
}

func newIPv4TestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on test port: %v", err)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}
