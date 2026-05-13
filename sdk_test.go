package qbt

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBuildCategoryRequest(t *testing.T) {
	t.Parallel()

	method, endpoint, body, headers := buildCategoryRequest("http://qb.test", "hash-1", "movies")

	if method != http.MethodPost {
		t.Fatalf("unexpected method: %s", method)
	}
	if endpoint != "http://qb.test/api/v2/torrents/setCategory" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if headers["Content-Type"] != "application/x-www-form-urlencoded" {
		t.Fatalf("unexpected content type: %s", headers["Content-Type"])
	}
	payload := string(body)
	if !strings.Contains(payload, "hashes=hash-1") || !strings.Contains(payload, "category=movies") {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestBuildCategoryRequestForRemoval(t *testing.T) {
	t.Parallel()

	_, _, body, _ := buildCategoryRequest("http://qb.test", "hash-1", "")
	payload := string(body)
	if !strings.Contains(payload, "hashes=hash-1") || !strings.Contains(payload, "category=") {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestHandleCategoryResponse_PropagatesHTTPError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("category rejected")),
		Header:     make(http.Header),
	}

	err := handleCategoryResponse(resp, "set category for torrent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "category rejected") {
		t.Fatalf("expected error body to be propagated, got %v", err)
	}
}
