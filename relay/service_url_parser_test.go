package relay

import (
	"testing"

	"github.com/unblink/unblink/shared"
)

func TestParseServiceURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expected     *shared.ServiceURL
		expectError  bool
		errorMessage string
	}{
		{
			name: "RTSP with auth",
			url:  "rtsp://admin:password@192.168.1.100:554/stream",
			expected: &shared.ServiceURL{
				Scheme:   "rtsp",
				Host:     "192.168.1.100",
				Port:     554,
				Path:     "/stream",
				Username: "admin",
				Password: "password",
			},
			expectError: false,
		},
		{
			name: "RTSP without auth and default port",
			url:  "rtsp://192.168.1.100/stream",
			expected: &shared.ServiceURL{
				Scheme:   "rtsp",
				Host:     "192.168.1.100",
				Port:     554,
				Path:     "/stream",
				Username: "",
				Password: "",
			},
			expectError: false,
		},
		{
			name: "HTTP with auth",
			url:  "http://admin:pass@192.168.1.100:8080/video",
			expected: &shared.ServiceURL{
				Scheme:   "http",
				Host:     "192.168.1.100",
				Port:     8080,
				Path:     "/video",
				Username: "admin",
				Password: "pass",
			},
			expectError: false,
		},
		{
			name: "HTTP without auth",
			url:  "http://192.168.1.100:8080/video",
			expected: &shared.ServiceURL{
				Scheme:   "http",
				Host:     "192.168.1.100",
				Port:     8080,
				Path:     "/video",
				Username: "",
				Password: "",
			},
			expectError: false,
		},
		{
			name: "HTTP with default port",
			url:  "http://example.com/api/stream",
			expected: &shared.ServiceURL{
				Scheme:   "http",
				Host:     "example.com",
				Port:     80,
				Path:     "/api/stream",
				Username: "",
				Password: "",
			},
			expectError: false,
		},
		{
			name: "HTTPS with auth",
			url:  "https://user:pass@example.com:443/path",
			expected: &shared.ServiceURL{
				Scheme:   "https",
				Host:     "example.com",
				Port:     443,
				Path:     "/path",
				Username: "user",
				Password: "pass",
			},
			expectError: false,
		},
		{
			name: "Empty URL",
			url:  "",
			expected: &shared.ServiceURL{
				Scheme:   "",
				Host:     "",
				Port:     0,
				Path:     "",
				Username: "",
				Password: "",
			},
			expectError:  true,
			errorMessage: "service URL cannot be empty",
		},
		{
			name:         "Missing scheme",
			url:          "192.168.1.100:554/stream",
			expectError:  true,
			errorMessage: "URL must include a protocol",
		},
		{
			name:         "Invalid port",
			url:          "rtsp://192.168.1.100:99999/stream",
			expectError:  true,
			errorMessage: "port must be between 1 and 65535",
		},
		{
			name:         "Unsupported protocol",
			url:          "ftp://example.com/file",
			expectError:  true,
			errorMessage: "unsupported protocol 'ftp'",
		},
		{
			name:         "Missing host",
			url:          "rtsp://:554/stream",
			expectError:  true,
			errorMessage: "URL must include a host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shared.ParseServiceURL(tt.url)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.errorMessage != "" {
					if !stringsContains(err.Error(), tt.errorMessage) {
						t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMessage, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result.Scheme != tt.expected.Scheme {
					t.Errorf("Expected scheme %s, got %s", tt.expected.Scheme, result.Scheme)
				}
				if result.Host != tt.expected.Host {
					t.Errorf("Expected host %s, got %s", tt.expected.Host, result.Host)
				}
				if result.Port != tt.expected.Port {
					t.Errorf("Expected port %d, got %d", tt.expected.Port, result.Port)
				}
				if result.Path != tt.expected.Path {
					t.Errorf("Expected path %s, got %s", tt.expected.Path, result.Path)
				}
				if result.Username != tt.expected.Username {
					t.Errorf("Expected username %s, got %s", tt.expected.Username, result.Username)
				}
				if result.Password != tt.expected.Password {
					t.Errorf("Expected password %s, got %s", tt.expected.Password, result.Password)
				}
			}
		})
	}
}

func TestDefaultPorts(t *testing.T) {
	tests := []struct {
		scheme   string
		expected int
	}{
		{"rtsp", 554},
		{"http", 80},
		{"https", 443},
	}

	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			if port, ok := shared.DefaultPorts[tt.scheme]; !ok || port != tt.expected {
				t.Errorf("Expected default port %d for %s, got %d", tt.expected, tt.scheme, port)
			}
		})
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
