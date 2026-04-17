// SPDX-License-Identifier: GPL-3.0-only
package qbt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/jfxdev/go-qbt/request"
)

// Default configuration constants
const (
	DefaultRequestTimeout  = 30 * time.Second
	DefaultMaxRetries      = 3
	DefaultRetryBackoff    = 1 * time.Second
	DefaultMaxDelay        = 30 * time.Second
	DefaultBackoffFactor   = 2.0
	DefaultMaxLoginRetries = 5
	CookieExpiryDuration   = 24 * time.Hour
	CookieCheckInterval    = 5 * time.Minute
)

func New(config Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating cookie jar: %w", err)
	}

	// Apply default configuration if not provided
	if config.RequestTimeout == 0 {
		config.RequestTimeout = DefaultRequestTimeout
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = DefaultMaxRetries
	}
	if config.RetryBackoff == 0 {
		config.RetryBackoff = DefaultRetryBackoff
	}
	if config.MaxLoginRetries == 0 {
		config.MaxLoginRetries = DefaultMaxLoginRetries
	}

	config.jar = jar

	client := &Client{
		config:          config,
		client:          &http.Client{Jar: jar, Timeout: config.RequestTimeout},
		MaxLoginRetries: config.MaxLoginRetries,
		RetryDelay:      2 * time.Second,
		cookieCache:     newCookieCache(),
		retryConfig:     newRetryConfig(config),
		lastLoginTime:   time.Time{},
		cookieValid:     false,
		status:          StatusInitializing,
		authFailed:      false,
	}

	// Start periodic cookie cleanup routine
	go client.startCookieCleanup()

	return client, nil
}

func (qb *Client) Update(config Config) {
	qb.mu.Lock()

	// Update runtime configuration
	qb.config = config
	if config.RequestTimeout > 0 {
		qb.client.Timeout = config.RequestTimeout
	}
	qb.mu.Unlock()

	// Invalidate cookies to force re-login
	qb.invalidateCookies()
}

func (qb *Client) Status() string {
	qb.mu.RLock()
	defer qb.mu.RUnlock()
	return qb.status
}

// checkAccessibility verifies that the qBittorrent API is accessible before attempting login.
// This helps distinguish between infrastructure issues (502, 503) and authentication issues.
func (qb *Client) checkAccessibility(ctx context.Context) error {
	// Use the version endpoint which doesn't require authentication
	checkCtx, cancel := context.WithTimeout(ctx, qb.config.RequestTimeout)
	defer cancel()

	resp, err := request.Do(http.MethodGet,
		fmt.Sprintf("%s/api/v2/app/version", qb.config.BaseURL),
		request.WithContext(checkCtx),
	)
	if err != nil {
		// Classify network-level errors
		clientErr := ClassifyError(err)
		qb.setStatus(StatusUnaccessible)
		qb.SetLastError(clientErr)
		return clientErr
	}
	defer resp.Body.Close()

	// Check for HTTP-level errors that indicate infrastructure issues
	if resp.StatusCode >= 500 {
		clientErr := classifyHTTPStatusCode(resp.StatusCode, "")
		qb.setStatus(StatusUnaccessible)
		qb.SetLastError(clientErr)
		return clientErr
	}

	// API is accessible
	return nil
}

// Login attempts to authenticate with the qBittorrent API.
// It can be used for an explicit startup check or on-demand re-authentication.
func (qb *Client) Login(ctx context.Context) error {
	return qb.loginWithContext(ctx)
}

func (qb *Client) loginWithContext(ctx context.Context) error {
	// Check if auth has permanently failed - don't attempt login
	if qb.IsAuthFailed() {
		return NewClientError(
			ErrorCodeAuthFailure,
			fmt.Sprintf("Authentication permanently failed after %d consecutive attempts - update credentials and restart", qb.config.MaxLoginRetries),
			nil,
			true,
		)
	}

	// First check if the API is accessible before attempting login
	if err := qb.checkAccessibility(ctx); err != nil {
		return err
	}

	data := url.Values{
		"username": {qb.config.Username},
		"password": {qb.config.Password},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	// Create a timeout context for the login request
	loginCtx, cancel := context.WithTimeout(ctx, qb.config.RequestTimeout)
	defer cancel()

	resp, err := request.Do(http.MethodPost,
		fmt.Sprintf("%s/api/v2/auth/login", qb.config.BaseURL),
		request.WithBody(strings.NewReader(data.Encode())),
		request.WithHeaders(headers),
		request.WithCookieJar(qb.config.jar),
		request.WithUpdateCookies(),
		request.WithContext(loginCtx),
	)
	if err != nil {
		// Classify the error
		clientErr := ClassifyError(err)
		qb.setStatus(StatusUnaccessible)
		qb.SetLastError(clientErr)

		// Return the classified error
		return clientErr
	}

	defer resp.Body.Close()

	// Read response body to check for "Fails." message
	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusOK {
		clientErr := classifyHTTPStatusCode(resp.StatusCode, bodyStr)
		qb.SetLastError(clientErr)
		return clientErr
	}

	// Check if response contains "Fails." message even with 200 status
	if strings.Contains(bodyStr, "Fails.") {
		qb.setStatus(StatusUnauthorized)

		// Increment consecutive failure counter
		count := qb.incrementLoginFailCount()
		permanent := count >= qb.config.MaxLoginRetries

		if permanent {
			qb.setAuthFailed(true)
		}

		clientErr := NewClientError(
			ErrorCodeAuthFailure,
			fmt.Sprintf("Invalid username or password (attempt %d/%d)", count, qb.config.MaxLoginRetries),
			nil,
			permanent,
		)
		qb.SetLastError(clientErr)

		if qb.config.Debug {
			log.Printf("Login auth failure %d/%d, permanent=%v", count, qb.config.MaxLoginRetries, permanent)
		}
		return clientErr
	}

	// Update cookie cache and mark as valid
	qb.updateCookieCache(resp.Cookies())
	qb.setCookieValid(true)
	qb.lastLoginTime = time.Now()
	qb.setStatus(StatusConnected)
	qb.SetLastError(nil) // Clear any previous error
	qb.resetLoginFailCount()

	if qb.config.Debug {
		log.Println("Login successful, cookies cached")
	}
	return nil
}

func (qb *Client) ensureLoginWithContext(ctx context.Context) error {
	// Use cached validity to avoid unnecessary requests
	if qb.isCookieValidCached() {
		return nil
	}

	// Try login with smart retry and context
	return qb.retryWithBackoffWithContext(ctx, func() error {
		return qb.loginWithContext(ctx)
	}, "login")
}

func (qb *Client) isCookieValid() bool {
	// Use cache first to avoid unnecessary requests
	if qb.isCookieValidCached() {
		return true
	}

	// Verify if cookies are expired
	if qb.isCookieExpired() {
		qb.setCookieValid(false)
		return false
	}

	// Do a lightweight validation request only if required
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := request.Do(http.MethodGet,
		fmt.Sprintf("%s/api/v2/app/version", qb.config.BaseURL),
		request.WithCookieJar(qb.config.jar),
		request.WithContext(ctx),
	)

	if err != nil || resp.StatusCode != http.StatusOK {
		qb.setCookieValid(false)
		return false
	}

	defer resp.Body.Close()

	// Update cache and mark as valid
	qb.updateCookieCache(resp.Cookies())
	qb.setCookieValid(true)
	return true
}

func (qb *Client) isCookieValidCached() bool {
	qb.cookieValidMu.RLock()
	defer qb.cookieValidMu.RUnlock()
	return qb.cookieValid
}

func (qb *Client) setCookieValid(valid bool) {
	qb.cookieValidMu.Lock()
	defer qb.cookieValidMu.Unlock()
	qb.cookieValid = valid
}

func (qb *Client) setStatus(status string) {
	qb.mu.Lock()
	defer qb.mu.Unlock()
	qb.status = status
}

func (qb *Client) GetStatus() string {
	qb.mu.RLock()
	defer qb.mu.RUnlock()
	return qb.status
}

// SetLastError stores the last error that occurred
func (qb *Client) SetLastError(err error) {
	qb.lastErrorMu.Lock()
	defer qb.lastErrorMu.Unlock()

	if err == nil {
		qb.lastError = nil
		return
	}

	// Classify the error
	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		qb.lastError = clientErr
	} else {
		qb.lastError = ClassifyError(err)
	}
}

// GetLastError returns the last error that occurred
func (qb *Client) GetLastError() *ClientError {
	qb.lastErrorMu.RLock()
	defer qb.lastErrorMu.RUnlock()
	return qb.lastError
}

// RefreshConnectionStatus attempts to refresh the connection status by performing
// a login if the client is not currently connected. This is intended to be called
// by health check endpoints so they return fresh (not stale/cached) status.
// It uses a short timeout and no retries to avoid blocking the caller.
func (qb *Client) RefreshConnectionStatus(ctx context.Context) *ConnectionStatus {
	currentStatus := qb.GetStatus()

	// If already connected with valid cookies, just return current status
	if currentStatus == StatusConnected && qb.isCookieValidCached() {
		return qb.GetConnectionStatus()
	}

	// If auth has permanently failed, don't attempt refresh
	if qb.IsAuthFailed() {
		return qb.GetConnectionStatus()
	}

	// Attempt a fresh login with a short timeout to get real status
	refreshCtx, cancel := context.WithTimeout(ctx, qb.config.RequestTimeout)
	defer cancel()

	if err := qb.loginWithContext(refreshCtx); err != nil {
		if qb.config.Debug {
			log.Printf("RefreshConnectionStatus: login failed: %v", err)
		}
		// Status and lastError were already updated by loginWithContext
		return qb.GetConnectionStatus()
	}

	return qb.GetConnectionStatus()
}

// GetConnectionStatus returns the detailed connection status
func (qb *Client) GetConnectionStatus() *ConnectionStatus {
	status := &ConnectionStatus{
		Status: qb.GetStatus(),
	}

	if lastErr := qb.GetLastError(); lastErr != nil {
		status.ErrorCode = lastErr.Code
		status.Message = lastErr.Message
		status.Permanent = lastErr.Permanent
	}

	return status
}

// setAuthFailed sets the authentication failure flag
func (qb *Client) setAuthFailed(failed bool) {
	qb.authFailedMu.Lock()
	defer qb.authFailedMu.Unlock()
	qb.authFailed = failed
}

// IsAuthFailed returns true if authentication has permanently failed
func (qb *Client) IsAuthFailed() bool {
	qb.authFailedMu.RLock()
	defer qb.authFailedMu.RUnlock()
	return qb.authFailed
}

// ResetAuthFailure resets the authentication failure flag (call after credential update)
func (qb *Client) ResetAuthFailure() {
	qb.setAuthFailed(false)
	qb.resetLoginFailCount()
	qb.SetLastError(nil)
	qb.setStatus(StatusInitializing)
}

// incrementLoginFailCount increments and returns the new consecutive login failure count.
func (qb *Client) incrementLoginFailCount() int {
	qb.loginFailCountMu.Lock()
	defer qb.loginFailCountMu.Unlock()
	qb.loginFailCount++
	return qb.loginFailCount
}

// resetLoginFailCount resets the consecutive login failure counter to zero.
func (qb *Client) resetLoginFailCount() {
	qb.loginFailCountMu.Lock()
	defer qb.loginFailCountMu.Unlock()
	qb.loginFailCount = 0
}

func (qb *Client) isCookieExpired() bool {
	return time.Since(qb.lastLoginTime) > CookieExpiryDuration
}

func (qb *Client) invalidateCookies() {
	qb.setCookieValid(false)
	qb.cookieCache.clear()
	qb.setStatus(StatusUnauthorized)

	// Recreate cookie jar to ensure old cookies aren't used
	jar, err := cookiejar.New(nil)
	if err == nil {
		qb.mu.Lock()
		qb.config.jar = jar
		qb.client.Jar = jar
		qb.mu.Unlock()
	} else if qb.config.Debug {
		log.Printf("failed to recreate cookie jar: %v", err)
	}
}

func (qb *Client) updateCookieCache(cookies []*http.Cookie) {
	qb.cookieCache.update(cookies)
}

func (qb *Client) retryWithBackoff(operation func() error, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= qb.retryConfig.MaxRetries; attempt++ {
		if err := operation(); err == nil {
			if attempt > 0 && qb.config.Debug {
				log.Printf("%s succeeded after %d retries", operationName, attempt)
			}
			return nil
		} else {
			lastErr = err
		}

		if attempt < qb.retryConfig.MaxRetries {
			delay := qb.calculateBackoffDelay(attempt)
			if qb.config.Debug {
				log.Printf("%s failed (attempt %d/%d), retrying in %v: %v",
					operationName, attempt+1, qb.retryConfig.MaxRetries+1, delay, lastErr)
			}
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w",
		operationName, qb.retryConfig.MaxRetries+1, lastErr)
}

func (qb *Client) retryWithBackoffWithContext(ctx context.Context, operation func() error, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= qb.retryConfig.MaxRetries; attempt++ {
		// Check if context is cancelled before each attempt
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s cancelled: %w", operationName, ctx.Err())
		default:
		}

		// Check if auth has permanently failed
		if qb.IsAuthFailed() {
			return NewClientError(
				ErrorCodeAuthFailure,
				"Authentication has permanently failed - update credentials and restart",
				nil,
				true,
			)
		}

		if err := operation(); err == nil {
			if attempt > 0 && qb.config.Debug {
				log.Printf("%s succeeded after %d retries", operationName, attempt)
			}
			return nil
		} else {
			lastErr = err

			// Check if this is a permanent error - don't retry
			if IsPermanentError(err) {
				if qb.config.Debug {
					log.Printf("%s failed with permanent error, not retrying: %v", operationName, err)
				}
				return err
			}
		}

		if attempt < qb.retryConfig.MaxRetries {
			delay := qb.calculateBackoffDelay(attempt)
			if qb.config.Debug {
				log.Printf("%s failed (attempt %d/%d), retrying in %v: %v",
					operationName, attempt+1, qb.retryConfig.MaxRetries+1, delay, lastErr)
			}

			// Use context-aware sleep
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return fmt.Errorf("%s cancelled during retry: %w", operationName, ctx.Err())
			}
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w",
		operationName, qb.retryConfig.MaxRetries+1, lastErr)
}

func (qb *Client) calculateBackoffDelay(attempt int) time.Duration {
	delay := qb.retryConfig.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * qb.retryConfig.BackoffFactor)
		if delay > qb.retryConfig.MaxDelay {
			delay = qb.retryConfig.MaxDelay
			break
		}
	}
	return delay
}

func (qb *Client) startCookieCleanup() {
	ticker := time.NewTicker(CookieCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		if qb.isCookieExpired() {
			qb.setCookieValid(false)
			qb.cookieCache.clear()
			// Do NOT change status to StatusUnauthorized here.
			// Cookie expiry is a normal lifecycle event — the next API call
			// will trigger re-authentication via ensureLoginWithContext.
			// Setting StatusUnauthorized here caused stale "auth failed" reports
			// when the health endpoint read cached status.
			if qb.config.Debug {
				log.Println("Cookies expired, cleared from cache — next request will re-authenticate")
			}
		}
	}
}

func (qb *Client) Close() error {
	// If client is not fully configured, just clear cache
	if qb.config.BaseURL == "" || qb.config.jar == nil {
		qb.invalidateCookies()
		return nil
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	ctx, cancel := context.WithTimeout(context.Background(), qb.config.RequestTimeout)
	defer cancel()

	resp, err := request.Do(http.MethodPost,
		fmt.Sprintf("%s/api/v2/auth/logout", qb.config.BaseURL),
		request.WithCookieJar(qb.config.jar),
		request.WithHeaders(headers),
		request.WithContext(ctx),
	)
	if err != nil {
		// Even on error, ensure local cache is cleared
		qb.invalidateCookies()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		qb.invalidateCookies()
		return fmt.Errorf("logout failed. Status: %d, Response: %s", resp.StatusCode, body)
	}

	// Clear cookie cache on logout
	qb.invalidateCookies()
	return nil
}

// Helpers for cookie cache
func newCookieCache() *CookieCache {
	return &CookieCache{
		cookies:    make(map[string]*http.Cookie),
		expiryTime: time.Now().Add(CookieExpiryDuration),
		lastUsed:   time.Now(),
	}
}

func (cc *CookieCache) update(cookies []*http.Cookie) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	for _, cookie := range cookies {
		cc.cookies[cookie.Name] = cookie
	}
	cc.lastUsed = time.Now()
}

func (cc *CookieCache) clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cookies = make(map[string]*http.Cookie)
	cc.expiryTime = time.Now().Add(CookieExpiryDuration)
	cc.lastUsed = time.Now()
}

func newRetryConfig(config Config) *RetryConfig {
	return &RetryConfig{
		MaxRetries:     config.MaxRetries,
		BaseDelay:      config.RetryBackoff,
		MaxDelay:       DefaultMaxDelay,
		BackoffFactor:  DefaultBackoffFactor,
		RetryableCodes: []int{408, 429, 500, 502, 503, 504},
	}
}
