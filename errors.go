package qbt

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrorCode represents a specific error type for client-side handling
type ErrorCode string

const (
	// ErrorCodeNone indicates no error
	ErrorCodeNone ErrorCode = ""

	// ErrorCodeAuthFailure indicates invalid username/password - requires user intervention
	ErrorCodeAuthFailure ErrorCode = "AUTH_FAILURE"

	// ErrorCodeTimeout indicates connection or request timeout - temporary, can retry
	ErrorCodeTimeout ErrorCode = "TIMEOUT"

	// ErrorCodeDNS indicates DNS resolution failure - check hostname configuration
	ErrorCodeDNS ErrorCode = "DNS_ERROR"

	// ErrorCodeHTTPSRequired indicates HTTP was used but HTTPS is required
	ErrorCodeHTTPSRequired ErrorCode = "HTTPS_REQUIRED"

	// ErrorCodeSSLError indicates SSL/TLS certificate or connection error
	ErrorCodeSSLError ErrorCode = "SSL_ERROR"

	// ErrorCodeVersionIncompatible indicates incompatible qBittorrent version
	ErrorCodeVersionIncompatible ErrorCode = "VERSION_INCOMPATIBLE"

	// ErrorCodeConnectionRefused indicates the server actively refused the connection
	ErrorCodeConnectionRefused ErrorCode = "CONNECTION_REFUSED"

	// ErrorCodeNetworkUnreachable indicates network routing issues
	ErrorCodeNetworkUnreachable ErrorCode = "NETWORK_UNREACHABLE"

	// ErrorCodeBadGateway indicates a proxy/gateway error (502)
	ErrorCodeBadGateway ErrorCode = "BAD_GATEWAY"

	// ErrorCodeServiceUnavailable indicates the service is temporarily unavailable (503)
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"

	// ErrorCodeUnknown indicates an unclassified error
	ErrorCodeUnknown ErrorCode = "UNKNOWN"
)

// ClientError represents a structured error with classification
type ClientError struct {
	Code    ErrorCode
	Message string
	Err     error
	// Permanent indicates whether this error requires user intervention (true)
	// or can be resolved by retrying (false)
	Permanent bool
}

func (e *ClientError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ClientError) Unwrap() error {
	return e.Err
}

// IsPermanent returns true if the error requires user intervention
func (e *ClientError) IsPermanent() bool {
	return e.Permanent
}

// NewClientError creates a new ClientError
func NewClientError(code ErrorCode, message string, err error, permanent bool) *ClientError {
	return &ClientError{
		Code:      code,
		Message:   message,
		Err:       err,
		Permanent: permanent,
	}
}

// ClassifyError analyzes an error and returns a structured ClientError
func ClassifyError(err error) *ClientError {
	if err == nil {
		return nil
	}

	// Already a ClientError
	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return clientErr
	}

	errStr := err.Error()

	// DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return NewClientError(
			ErrorCodeDNS,
			fmt.Sprintf("Failed to resolve hostname: %s", dnsErr.Name),
			err,
			true,
		)
	}

	// Network operation errors (connection refused, timeout, etc.)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return classifyOpError(opErr, err)
	}

	// URL errors
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// Check if it wraps another error we can classify
		if urlErr.Err != nil {
			if classified := ClassifyError(urlErr.Err); classified != nil {
				return classified
			}
		}

		// Timeout
		if urlErr.Timeout() {
			return NewClientError(
				ErrorCodeTimeout,
				"Request timed out",
				err,
				false,
			)
		}
	}

	// TLS/SSL errors
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return NewClientError(
			ErrorCodeSSLError,
			"SSL certificate verification failed",
			err,
			true,
		)
	}

	// Check for common error patterns in the error string
	return classifyByMessage(errStr, err)
}

// classifyOpError classifies net.OpError errors
func classifyOpError(opErr *net.OpError, originalErr error) *ClientError {
	// Connection refused
	if opErr.Op == "dial" {
		if strings.Contains(opErr.Error(), "connection refused") {
			return NewClientError(
				ErrorCodeConnectionRefused,
				"Connection refused - server may be down or port is incorrect",
				originalErr,
				false,
			)
		}

		if strings.Contains(opErr.Error(), "no route to host") ||
			strings.Contains(opErr.Error(), "network is unreachable") {
			return NewClientError(
				ErrorCodeNetworkUnreachable,
				"Network unreachable - check network connectivity",
				originalErr,
				false,
			)
		}
	}

	// Timeout
	if opErr.Timeout() {
		return NewClientError(
			ErrorCodeTimeout,
			"Connection timed out",
			originalErr,
			false,
		)
	}

	// Default network error
	return NewClientError(
		ErrorCodeUnknown,
		"Network operation failed",
		originalErr,
		false,
	)
}

// classifyByMessage classifies errors based on error message patterns
func classifyByMessage(errStr string, err error) *ClientError {
	lowerErr := strings.ToLower(errStr)

	// Timeout patterns
	if strings.Contains(lowerErr, "timeout") ||
		strings.Contains(lowerErr, "deadline exceeded") ||
		strings.Contains(lowerErr, "context canceled") {
		return NewClientError(
			ErrorCodeTimeout,
			"Request timed out",
			err,
			false,
		)
	}

	// SSL/TLS patterns
	if strings.Contains(lowerErr, "certificate") ||
		strings.Contains(lowerErr, "x509") ||
		strings.Contains(lowerErr, "tls") ||
		strings.Contains(lowerErr, "ssl") {
		return NewClientError(
			ErrorCodeSSLError,
			"SSL/TLS connection failed - check certificate configuration",
			err,
			true,
		)
	}

	// HTTP/HTTPS mismatch
	if strings.Contains(lowerErr, "malformed http response") ||
		strings.Contains(lowerErr, "first record does not look like a tls handshake") {
		return NewClientError(
			ErrorCodeHTTPSRequired,
			"Protocol mismatch - try using HTTPS instead of HTTP",
			err,
			true,
		)
	}

	// Connection refused
	if strings.Contains(lowerErr, "connection refused") {
		return NewClientError(
			ErrorCodeConnectionRefused,
			"Connection refused - server may be down",
			err,
			false,
		)
	}

	// DNS patterns
	if strings.Contains(lowerErr, "no such host") ||
		strings.Contains(lowerErr, "lookup") ||
		strings.Contains(lowerErr, "dns") {
		return NewClientError(
			ErrorCodeDNS,
			"DNS resolution failed - check hostname",
			err,
			true,
		)
	}

	// Authentication patterns
	// "Fails." is the specific message from qBittorrent API for invalid credentials
	if strings.Contains(lowerErr, "fails.") ||
		strings.Contains(lowerErr, "unauthorized") ||
		strings.Contains(lowerErr, "authentication failed") ||
		strings.Contains(lowerErr, "invalid username") ||
		strings.Contains(lowerErr, "invalid password") ||
		strings.Contains(lowerErr, "invalid credentials") {
		return NewClientError(
			ErrorCodeAuthFailure,
			"Invalid username or password",
			err,
			true,
		)
	}

	// Default unknown error
	return NewClientError(
		ErrorCodeUnknown,
		"Unknown error occurred",
		err,
		false,
	)
}

// IsRetryableError returns true if the error is temporary and can be retried
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return !clientErr.Permanent
	}

	// Classify and check
	classified := ClassifyError(err)
	return !classified.Permanent
}

// IsPermanentError returns true if the error requires user intervention
func IsPermanentError(err error) bool {
	if err == nil {
		return false
	}

	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return clientErr.Permanent
	}

	// Classify and check
	classified := ClassifyError(err)
	return classified.Permanent
}

// classifyHTTPStatusCode classifies an HTTP status code into a ClientError
func classifyHTTPStatusCode(statusCode int, body string) *ClientError {
	switch statusCode {
	case 401, 403:
		return NewClientError(
			ErrorCodeAuthFailure,
			fmt.Sprintf("Authentication failed with status %d", statusCode),
			nil,
			true,
		)
	case 502:
		return NewClientError(
			ErrorCodeBadGateway,
			fmt.Sprintf("Bad Gateway (502): %s", body),
			nil,
			false,
		)
	case 503:
		return NewClientError(
			ErrorCodeServiceUnavailable,
			fmt.Sprintf("Service Unavailable (503): %s", body),
			nil,
			false,
		)
	case 504:
		return NewClientError(
			ErrorCodeTimeout,
			fmt.Sprintf("Gateway Timeout (504): %s", body),
			nil,
			false,
		)
	default:
		return NewClientError(
			ErrorCodeUnknown,
			fmt.Sprintf("Request failed with status %d: %s", statusCode, body),
			nil,
			false,
		)
	}
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	if err == nil {
		return ErrorCodeNone
	}

	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return clientErr.Code
	}

	// Classify and return code
	classified := ClassifyError(err)
	return classified.Code
}
