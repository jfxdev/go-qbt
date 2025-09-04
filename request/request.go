package request

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Options that configure the HTTP request
type RequestOptions struct {
	Timeout        time.Duration
	Body           io.Reader
	Headers        map[string]string
	Ctx            context.Context
	CookieJar      http.CookieJar
	UpdateCookies  bool
	PreRequestHook func() error
	Method         string // HTTP method to use (GET, POST, etc.)
}

// Function type used to apply functional options to RequestOptions
type RequestOption func(*RequestOptions)

// WithMethod sets the HTTP method for the request
func WithMethod(method string) RequestOption {
	return func(o *RequestOptions) {
		o.Method = method
	}
}

// WithTimeout sets a timeout for the request (in seconds)
func WithTimeout(seconds int) RequestOption {
	return func(o *RequestOptions) {
		o.Timeout = time.Duration(seconds) * time.Second
	}
}

// WithTimeoutDuration sets a timeout using a time.Duration
func WithTimeoutDuration(duration time.Duration) RequestOption {
	return func(o *RequestOptions) {
		o.Timeout = duration
	}
}

// WithBody sets the request body
func WithBody(body io.Reader) RequestOption {
	return func(o *RequestOptions) {
		o.Body = body
	}
}

// WithHeader adds a header to the request
func WithHeader(key, value string) RequestOption {
	return func(o *RequestOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string)
		}
		o.Headers[key] = value
	}
}

// WithHeaders adds multiple headers at once
func WithHeaders(headers map[string]string) RequestOption {
	return func(o *RequestOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string)
		}
		for k, v := range headers {
			o.Headers[k] = v
		}
	}
}

// WithContext sets the context for the request
func WithContext(ctx context.Context) RequestOption {
	return func(o *RequestOptions) {
		o.Ctx = ctx
	}
}

// WithCookieJar sets the CookieJar used to persist cookies across requests
func WithCookieJar(jar http.CookieJar) RequestOption {
	return func(o *RequestOptions) {
		o.CookieJar = jar
	}
}

// WithUpdateCookies persists response cookies into the provided CookieJar
func WithUpdateCookies() RequestOption {
	return func(o *RequestOptions) {
		o.UpdateCookies = true
	}
}

// WithPreRequestHook sets a hook executed right before the request is sent
func WithPreRequestHook(hook func() error) RequestOption {
	return func(o *RequestOptions) {
		o.PreRequestHook = hook
	}
}

// Do executes an HTTP request with the provided options
func Do(method, url string, opts ...RequestOption) (*http.Response, error) {
	// Default options
	options := &RequestOptions{
		Timeout: 10 * time.Second, // 10s default
		Ctx:     context.Background(),
		Body:    nil,
		Method:  method, // use the provided method
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(options)
	}

	// Create an HTTP client with the configured timeout
	client := &http.Client{Timeout: options.Timeout}

	// Attach CookieJar if provided
	if options.CookieJar != nil {
		client.Jar = options.CookieJar
	}

	// Run pre-request hook if configured
	if options.PreRequestHook != nil {
		if err := options.PreRequestHook(); err != nil {
			return nil, err
		}
	}

	// Build the request using the proper method
	req, err := http.NewRequestWithContext(options.Ctx, options.Method, url, options.Body)
	if err != nil {
		return nil, err
	}

	// Add headers
	for k, v := range options.Headers {
		req.Header.Set(k, v)
	}

	// Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Save response cookies to the CookieJar if requested
	if options.UpdateCookies && options.CookieJar != nil {
		options.CookieJar.SetCookies(req.URL, resp.Cookies())
	}

	return resp, nil
}
