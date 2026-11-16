package routes

import (
	_ "github.com/Harsh7213/shorty/docs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSwaggerEndpoints(t *testing.T) {
	previousMux := http.DefaultServeMux
	http.DefaultServeMux = http.NewServeMux()
	t.Cleanup(func() {
		http.DefaultServeMux = previousMux
	})

	RegisterRoutes()

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/swagger/index.html", contentType: "text/html", contains: "Swagger UI"},
		{path: "/swagger/doc.json", contentType: "application/json", contains: "/api/v1/health"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			http.DefaultServeMux.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET %s returned status %d, want %d", test.path, response.Code, http.StatusOK)
			}
			if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, test.contentType) {
				t.Errorf("GET %s returned content type %q, want it to contain %q", test.path, contentType, test.contentType)
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Errorf("GET %s response does not contain %q", test.path, test.contains)
			}
		})
	}
}
