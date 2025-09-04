package qbt

import (
	"net/http"
	"testing"
	"time"
)

func BenchmarkCookieValidation(b *testing.B) {
	client := &Client{
		cookieValid: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.isCookieValidCached()
	}
}

func BenchmarkCookieValidationWithoutCache(b *testing.B) {
	client := &Client{
		cookieValid: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.isCookieValid()
	}
}

func BenchmarkRetryBackoffCalculation(b *testing.B) {
	client := &Client{
		retryConfig: &RetryConfig{
			BaseDelay:     1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.calculateBackoffDelay(i % 10)
	}
}

func BenchmarkRetryWithBackoff(b *testing.B) {
	client := &Client{
		retryConfig: &RetryConfig{
			MaxRetries:    3,
			BaseDelay:     1 * time.Millisecond,
			MaxDelay:      10 * time.Millisecond,
			BackoffFactor: 2.0,
		},
	}

	operation := func() error {
		return nil // Operation always succeeds
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.retryWithBackoff(operation, "benchmark")
	}
}

func BenchmarkCookieCacheUpdate(b *testing.B) {
	cache := newCookieCache()
	testCookies := []*http.Cookie{
		{Name: "cookie1", Value: "value1"},
		{Name: "cookie2", Value: "value2"},
		{Name: "cookie3", Value: "value3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.update(testCookies)
	}
}

func BenchmarkCookieCacheClear(b *testing.B) {
	cache := newCookieCache()

	// Pre-populate cache with some cookies
	testCookies := []*http.Cookie{
		{Name: "cookie1", Value: "value1"},
		{Name: "cookie2", Value: "value2"},
	}
	cache.update(testCookies)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.clear()
		// Restore cookies for next iteration
		cache.update(testCookies)
	}
}

func BenchmarkIsRetryableStatusCode(b *testing.B) {
	client := &Client{
		retryConfig: &RetryConfig{
			RetryableCodes: []int{408, 429, 500, 502, 503, 504},
		},
	}

	statusCodes := []int{200, 400, 408, 429, 500, 502, 503, 504}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.isRetryableStatusCode(statusCodes[i%len(statusCodes)])
	}
}

func BenchmarkClientCreation(b *testing.B) {
	config := Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   1 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := New(config)
		if err != nil {
			b.Fatalf("Failed to create client: %v", err)
		}
		client.Close()
	}
}

func BenchmarkConfigUpdate(b *testing.B) {
	client, err := New(Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	newConfig := Config{
		BaseURL:        "http://localhost:8080",
		Username:       "test",
		Password:       "test",
		RequestTimeout: 60 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.Update(newConfig)
	}
}

// Benchmark for concurrent operations
func BenchmarkConcurrentCookieValidation(b *testing.B) {
	client := &Client{
		cookieValid: true,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.isCookieValidCached()
		}
	})
}

func BenchmarkConcurrentRetryBackoff(b *testing.B) {
	client := &Client{
		retryConfig: &RetryConfig{
			MaxRetries:    2,
			BaseDelay:     1 * time.Millisecond,
			MaxDelay:      10 * time.Millisecond,
			BackoffFactor: 2.0,
		},
	}

	operation := func() error {
		return nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client.retryWithBackoff(operation, "concurrent")
		}
	})
}
