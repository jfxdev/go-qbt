// SPDX-License-Identifier: GPL-3.0-only
package qbt

import (
	"context"
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
	DefaultRequestTimeout = 30 * time.Second
	DefaultMaxRetries     = 3
	DefaultRetryBackoff   = 1 * time.Second
	DefaultMaxDelay       = 30 * time.Second
	DefaultBackoffFactor  = 2.0
	CookieExpiryDuration  = 24 * time.Hour
	CookieCheckInterval   = 5 * time.Minute
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

	config.jar = jar

	client := &Client{
		config:          config,
		client:          &http.Client{Jar: jar, Timeout: config.RequestTimeout},
		MaxLoginRetries: 3,
		RetryDelay:      2 * time.Second,
		cookieCache:     newCookieCache(),
		retryConfig:     newRetryConfig(config),
		lastLoginTime:   time.Time{},
		cookieValid:     false,
	}

	// Start periodic cookie cleanup routine
	go client.startCookieCleanup()

	return client, nil
}

func (qb *Client) Update(config Config) {
	qb.mu.Lock()
	defer qb.mu.Unlock()

	// Update runtime configuration
	qb.config = config
	if config.RequestTimeout > 0 {
		qb.client.Timeout = config.RequestTimeout
	}

	// Invalidate cookies to force re-login
	qb.invalidateCookies()
}

func (qb *Client) loginWithContext(ctx context.Context) error {
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
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed. Status: %d, Response: %s", resp.StatusCode, body)
	}

	// Update cookie cache and mark as valid
	qb.updateCookieCache(resp.Cookies())
	qb.setCookieValid(true)
	qb.lastLoginTime = time.Now()

	log.Println("Login successful, cookies cached")
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

func (qb *Client) ensureLoginSimple() error {
	// Use cached validity to avoid unnecessary requests
	if qb.isCookieValidCached() {
		return nil
	}

	// Try login with simple retry (no context)
	return qb.retryWithBackoff(func() error {
		return qb.loginWithContext(context.Background())
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

func (qb *Client) isCookieExpired() bool {
	return time.Since(qb.lastLoginTime) > CookieExpiryDuration
}

func (qb *Client) invalidateCookies() {
	qb.setCookieValid(false)
	qb.cookieCache.clear()
}

func (qb *Client) updateCookieCache(cookies []*http.Cookie) {
	qb.cookieCache.update(cookies)
}

func (qb *Client) retryWithBackoff(operation func() error, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= qb.retryConfig.MaxRetries; attempt++ {
		if err := operation(); err == nil {
			if attempt > 0 {
				log.Printf("%s succeeded after %d retries", operationName, attempt)
			}
			return nil
		} else {
			lastErr = err
		}

		if attempt < qb.retryConfig.MaxRetries {
			delay := qb.calculateBackoffDelay(attempt)
			log.Printf("%s failed (attempt %d/%d), retrying in %v: %v",
				operationName, attempt+1, qb.retryConfig.MaxRetries+1, delay, lastErr)
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

		if err := operation(); err == nil {
			if attempt > 0 {
				log.Printf("%s succeeded after %d retries", operationName, attempt)
			}
			return nil
		} else {
			lastErr = err
		}

		if attempt < qb.retryConfig.MaxRetries {
			delay := qb.calculateBackoffDelay(attempt)
			log.Printf("%s failed (attempt %d/%d), retrying in %v: %v",
				operationName, attempt+1, qb.retryConfig.MaxRetries+1, delay, lastErr)

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
			log.Println("Cookies expired, cleared from cache")
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
