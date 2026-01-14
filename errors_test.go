package qbt

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
)

func TestClassifyErrorNil(t *testing.T) {
	result := ClassifyError(nil)
	if result != nil {
		t.Errorf("Expected nil for nil error, got %v", result)
	}
}

func TestClassifyErrorClientError(t *testing.T) {
	original := NewClientError(ErrorCodeAuthFailure, "test message", nil, true)
	result := ClassifyError(original)

	if result.Code != ErrorCodeAuthFailure {
		t.Errorf("Expected ErrorCodeAuthFailure, got %v", result.Code)
	}
	if result.Message != "test message" {
		t.Errorf("Expected 'test message', got %v", result.Message)
	}
	if !result.Permanent {
		t.Error("Expected permanent to be true")
	}
}

func TestClassifyErrorDNS(t *testing.T) {
	dnsErr := &net.DNSError{
		Err:  "no such host",
		Name: "example.invalid",
	}
	result := ClassifyError(dnsErr)

	if result.Code != ErrorCodeDNS {
		t.Errorf("Expected ErrorCodeDNS, got %v", result.Code)
	}
	if !result.Permanent {
		t.Error("Expected DNS errors to be permanent")
	}
}

func TestClassifyErrorTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"context deadline exceeded", context.DeadlineExceeded},
		{"context canceled", context.Canceled},
		{"timeout string", errors.New("connection timeout")},
		{"deadline string", errors.New("deadline exceeded")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if result.Code != ErrorCodeTimeout {
				t.Errorf("Expected ErrorCodeTimeout for %s, got %v", tt.name, result.Code)
			}
			if result.Permanent {
				t.Error("Expected timeout errors to be temporary (not permanent)")
			}
		})
	}
}

func TestClassifyErrorAuth(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"fails message", errors.New("Fails. Invalid credentials")},
		{"unauthorized", errors.New("unauthorized access")},
		{"authentication failed", errors.New("authentication failed")},
		{"invalid username", errors.New("invalid username or password")},
		{"invalid credentials", errors.New("invalid credentials provided")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if result.Code != ErrorCodeAuthFailure {
				t.Errorf("Expected ErrorCodeAuthFailure for %s, got %v", tt.name, result.Code)
			}
			if !result.Permanent {
				t.Error("Expected auth errors to be permanent")
			}
		})
	}
}

func TestClassifyErrorSSL(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"certificate error string", errors.New("certificate verify failed")},
		{"tls error string", errors.New("tls: handshake failure")},
		{"ssl error string", errors.New("ssl error occurred")},
		{"x509 error string", errors.New("x509: certificate signed by unknown authority")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if result.Code != ErrorCodeSSLError {
				t.Errorf("Expected ErrorCodeSSLError for %s, got %v", tt.name, result.Code)
			}
			if !result.Permanent {
				t.Error("Expected SSL errors to be permanent")
			}
		})
	}
}

func TestClassifyErrorTLSCertificateVerification(t *testing.T) {
	certErr := &tls.CertificateVerificationError{
		Err: errors.New("certificate verification failed"),
	}
	result := ClassifyError(certErr)

	if result.Code != ErrorCodeSSLError {
		t.Errorf("Expected ErrorCodeSSLError, got %v", result.Code)
	}
	if !result.Permanent {
		t.Error("Expected TLS cert errors to be permanent")
	}
}

func TestClassifyErrorConnectionRefused(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"connection refused string", errors.New("connection refused")},
		{"dial connection refused", errors.New("dial tcp: connection refused")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if result.Code != ErrorCodeConnectionRefused {
				t.Errorf("Expected ErrorCodeConnectionRefused for %s, got %v", tt.name, result.Code)
			}
			if result.Permanent {
				t.Error("Expected connection refused errors to be temporary")
			}
		})
	}
}

func TestClassifyErrorHTTPSRequired(t *testing.T) {
	// Test malformed http response (clear HTTPS required case)
	result := ClassifyError(errors.New("malformed http response"))
	if result.Code != ErrorCodeHTTPSRequired {
		t.Errorf("Expected ErrorCodeHTTPSRequired for malformed http, got %v", result.Code)
	}
	if !result.Permanent {
		t.Error("Expected HTTPS required errors to be permanent")
	}
}

func TestClassifyErrorNetOpError(t *testing.T) {
	// Test connection refused via OpError
	opErr := &net.OpError{
		Op:  "dial",
		Err: errors.New("connection refused"),
	}
	result := ClassifyError(opErr)

	if result.Code != ErrorCodeConnectionRefused {
		t.Errorf("Expected ErrorCodeConnectionRefused, got %v", result.Code)
	}
}

func TestClassifyErrorNetworkUnreachable(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Err: errors.New("no route to host"),
	}
	result := ClassifyError(opErr)

	if result.Code != ErrorCodeNetworkUnreachable {
		t.Errorf("Expected ErrorCodeNetworkUnreachable, got %v", result.Code)
	}
}

func TestClassifyHTTPStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectedCode ErrorCode
		permanent    bool
	}{
		{"401 unauthorized", 401, ErrorCodeAuthFailure, true},
		{"403 forbidden", 403, ErrorCodeAuthFailure, true},
		{"502 bad gateway", 502, ErrorCodeBadGateway, false},
		{"503 service unavailable", 503, ErrorCodeServiceUnavailable, false},
		{"504 gateway timeout", 504, ErrorCodeTimeout, false},
		{"500 internal server error", 500, ErrorCodeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyHTTPStatusCode(tt.statusCode, "test body")
			if result.Code != tt.expectedCode {
				t.Errorf("Expected %v for %s, got %v", tt.expectedCode, tt.name, result.Code)
			}
			if result.Permanent != tt.permanent {
				t.Errorf("Expected permanent=%v for %s, got %v", tt.permanent, tt.name, result.Permanent)
			}
		})
	}
}

func TestClassifyErrorUnknown(t *testing.T) {
	err := errors.New("some random error that doesn't match any pattern")
	result := ClassifyError(err)

	if result.Code != ErrorCodeUnknown {
		t.Errorf("Expected ErrorCodeUnknown, got %v", result.Code)
	}
	if result.Permanent {
		t.Error("Expected unknown errors to be temporary by default")
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("connection timeout"), true},
		{"auth error", errors.New("Fails. Invalid credentials"), false},
		{"connection refused", errors.New("connection refused"), true},
		{"dns error", &net.DNSError{Err: "no such host", Name: "test"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("Expected IsRetryableError=%v for %s, got %v", tt.retryable, tt.name, result)
			}
		})
	}
}

func TestIsPermanentError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{"nil error", nil, false},
		{"timeout error", errors.New("connection timeout"), false},
		{"auth error", errors.New("login failed"), true},
		{"connection refused", errors.New("connection refused"), false},
		{"dns error", &net.DNSError{Err: "no such host", Name: "test"}, true},
		{"ssl error", errors.New("certificate verify failed"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPermanentError(tt.err)
			if result != tt.permanent {
				t.Errorf("Expected IsPermanentError=%v for %s, got %v", tt.permanent, tt.name, result)
			}
		})
	}
}

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{"nil error", nil, ErrorCodeNone},
		{"timeout error", errors.New("connection timeout"), ErrorCodeTimeout},
		{"auth error", errors.New("login failed"), ErrorCodeAuthFailure},
		{"client error", NewClientError(ErrorCodeVersionIncompatible, "test", nil, true), ErrorCodeVersionIncompatible},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorCode(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v for %s, got %v", tt.expected, tt.name, result)
			}
		})
	}
}

func TestClientErrorImplementsError(t *testing.T) {
	err := NewClientError(ErrorCodeAuthFailure, "test message", nil, true)

	// Check that it implements error interface
	var _ error = err

	// Check error message format
	errStr := err.Error()
	if errStr != "AUTH_FAILURE: test message" {
		t.Errorf("Unexpected error message: %s", errStr)
	}
}

func TestClientErrorWithWrappedError(t *testing.T) {
	wrapped := errors.New("underlying error")
	err := NewClientError(ErrorCodeTimeout, "test message", wrapped, false)

	// Check error message format
	errStr := err.Error()
	expected := "TIMEOUT: test message (underlying error)"
	if errStr != expected {
		t.Errorf("Expected %q, got %q", expected, errStr)
	}

	// Check unwrap
	if errors.Unwrap(err) != wrapped {
		t.Error("Expected Unwrap to return wrapped error")
	}
}
