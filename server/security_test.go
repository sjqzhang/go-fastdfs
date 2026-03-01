package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPathTraversalProtection tests the isSafePath function
// against various path traversal attack patterns
func TestPathTraversalProtection(t *testing.T) {
	// Initialize server for testing
	if server == nil {
		InitServer()
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Normal paths should be allowed
		{"normal file path", "/group1/default/20260227/file.txt", true},
		{"normal path with subdirs", "/group1/default/20260227/12/34/file.txt", true},

		// Path traversal attacks should be blocked
		{"double dot", "/group1/../secret/file.txt", false},
		{"double dot encoded", "/group1/%2e%2e/secret/file.txt", false},
		{"double dot double encoded", "/group1/%252e%252e/secret/file.txt", false},
		{"mixed case encoded", "/group1/%2E%2E/secret/file.txt", false},

		// Other dangerous patterns
		{"null byte", "/group1/file.txt%00.jpg", false},
		{"null byte encoded", "/group1/file.txt%2500.jpg", false},
		{"home directory", "/group1/~/secret/file.txt", false},
		{"double slash", "/group1//default/file.txt", true}, // filepath.Clean handles this

		// Edge cases
		{"empty path", "", true},
		{"root path", "/", true},
		{"group only", "/group1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.isSafePath(tt.path)
			if result != tt.expected {
				t.Errorf("isSafePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestDownloadPathValidation tests the download endpoint
// for proper path validation
func TestDownloadPathValidation(t *testing.T) {
	// Initialize server for testing
	if server == nil {
		InitServer()
	}

	maliciousPaths := []string{
		"/group1/../conf/cfg.json",
		"/group1/%2e%2e/conf/cfg.json",
		"/group1/../../../etc/passwd",
	}

	for _, path := range maliciousPaths {
		t.Run("path: "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			server.Download(w, req)

			// Should return 401 (NotPermit) for malicious paths
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusNotFound {
				t.Errorf("Expected 401 or 404 for malicious path %q, got %d", path, w.Code)
			}
		})
	}
}

// TestFilePermissionConstants verifies the permission constants are set correctly
func TestFilePermissionConstants(t *testing.T) {
	if DEFAULT_DIR_PERM != 0755 {
		t.Errorf("DEFAULT_DIR_PERM = %o, want %o", DEFAULT_DIR_PERM, 0755)
	}

	if DEFAULT_FILE_PERM != 0644 {
		t.Errorf("DEFAULT_FILE_PERM = %o, want %o", DEFAULT_FILE_PERM, 0644)
	}
}
