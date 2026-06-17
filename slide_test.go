package slidown

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewImage(t *testing.T) {
	tests := []struct {
		name         string
		pathOrURL    string
		expectedType MIMEType
		expectError  bool
	}{
		{
			name:         "load PNG image file",
			pathOrURL:    "testdata/test.png",
			expectedType: MIMETypeImagePNG,
			expectError:  false,
		},
		{
			name:         "load JPEG image file",
			pathOrURL:    "testdata/test.jpeg",
			expectedType: MIMETypeImageJPEG,
			expectError:  false,
		},
		{
			name:         "load GIF image file",
			pathOrURL:    "testdata/test.gif",
			expectedType: MIMETypeImageGIF,
			expectError:  false,
		},
		{
			name:        "non-existent file",
			pathOrURL:   "testdata/nonexistent.png",
			expectError: true,
		},
		{
			name:        "invalid file path",
			pathOrURL:   "invalid/path/image.png",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := NewImage(tt.pathOrURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error occurred: %v", err)
				return
			}

			if img == nil {
				t.Error("Image is nil")
				return
			}

			if img.mimeType != tt.expectedType {
				t.Errorf("MIME type mismatch. Expected: %s, Got: %s", tt.expectedType, img.mimeType)
			}

			if _, err := img.Image(); err != nil {
				t.Errorf("Failed to decode image: %v", err)
			}

			// Check basic image properties
			bounds := img.i.Bounds()
			if bounds.Empty() {
				t.Error("Image bounds are empty")
			}
		})
	}
}

func TestNewImageFromURL(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test.png":
			// Read testdata/test.png content and return as response
			data, err := os.ReadFile("testdata/test.png")
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(data)
		case "/test.jpeg":
			data, err := os.ReadFile("testdata/test.jpeg")
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(data)
		case "/notfound":
			http.Error(w, "Not Found", http.StatusNotFound)
		default:
			http.Error(w, "Bad Request", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	tests := []struct {
		name         string
		url          string
		expectedType MIMEType
		expectError  bool
	}{
		{
			name:         "fetch PNG image via HTTP",
			url:          server.URL + "/test.png",
			expectedType: MIMETypeImagePNG,
			expectError:  false,
		},
		{
			name:         "fetch JPEG image via HTTP",
			url:          server.URL + "/test.jpeg",
			expectedType: MIMETypeImageJPEG,
			expectError:  false,
		},
		{
			name:        "non-existent URL",
			url:         server.URL + "/notfound",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := NewImage(tt.url)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error occurred: %v", err)
				return
			}

			if img == nil {
				t.Error("Image is nil")
				return
			}

			if img.mimeType != tt.expectedType {
				t.Errorf("MIME type mismatch. Expected: %s, Got: %s", tt.expectedType, img.mimeType)
			}
		})
	}
}

func TestImageString(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "PNG image String() method",
			filePath: "testdata/test.png",
		},
		{
			name:     "JPEG image String() method",
			filePath: "testdata/test.jpeg",
		},
		{
			name:     "GIF image String() method",
			filePath: "testdata/test.gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := NewImage(tt.filePath)
			if err != nil {
				t.Fatalf("Failed to load image: %v", err)
			}

			got := img.String()

			expectedPrefix := fmt.Sprintf("data:%s;base64,", img.mimeType)
			if len(got) < len(expectedPrefix) || got[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("String() format is incorrect. Expected prefix: %s", expectedPrefix)
			}
		})
	}
}

func TestImageBytes(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
	}{
		{
			name:     "PNG image Bytes() method",
			filePath: "testdata/test.png",
		},
		{
			name:     "JPEG image Bytes() method",
			filePath: "testdata/test.jpeg",
		},
		{
			name:     "GIF image Bytes() method",
			filePath: "testdata/test.gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := NewImage(tt.filePath)
			if err != nil {
				t.Fatalf("failed to load image: %v", err)
			}

			bytes := img.Bytes()
			if len(bytes) == 0 {
				t.Error("Bytes() method returned empty byte array")
			}
		})
	}
}
