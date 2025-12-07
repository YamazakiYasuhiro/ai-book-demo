package utils

import (
	"net/http"
	"testing"
)

// AssertStatusCode checks if response has expected status code
func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("Expected status code %d, got %d", expected, resp.StatusCode)
	}
}

// AssertStatusOK checks if response has 200 OK status
func AssertStatusOK(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertStatusCode(t, resp, http.StatusOK)
}

// AssertStatusCreated checks if response has 201 Created status
func AssertStatusCreated(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertStatusCode(t, resp, http.StatusCreated)
}

// AssertStatusNoContent checks if response has 204 No Content status
func AssertStatusNoContent(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertStatusCode(t, resp, http.StatusNoContent)
}

// AssertStatusBadRequest checks if response has 400 Bad Request status
func AssertStatusBadRequest(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertStatusCode(t, resp, http.StatusBadRequest)
}

// AssertStatusNotFound checks if response has 404 Not Found status
func AssertStatusNotFound(t *testing.T, resp *http.Response) {
	t.Helper()
	AssertStatusCode(t, resp, http.StatusNotFound)
}

// AssertContentType checks if response has expected content type
func AssertContentType(t *testing.T, resp *http.Response, expected string) {
	t.Helper()
	contentType := resp.Header.Get("Content-Type")
	if contentType != expected {
		t.Errorf("Expected Content-Type %q, got %q", expected, contentType)
	}
}

// AssertJSONContentType checks if response has JSON content type
func AssertJSONContentType(t *testing.T, resp *http.Response) {
	t.Helper()
	contentType := resp.Header.Get("Content-Type")
	// Allow for charset suffix
	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected JSON Content-Type, got %q", contentType)
	}
}

