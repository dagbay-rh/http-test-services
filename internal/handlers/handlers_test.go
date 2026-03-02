package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	RegisterRoutes(mux, "../../docs")
	return mux
}

// routesFor returns the root and console-prefixed variants of a path.
func routesFor(path string) []string {
	return []string{
		"/" + path,
		ApiPrefix + "/" + path,
	}
}

func TestRootRedirect(t *testing.T) {
	mux := setupMux(t)
	for _, path := range []string{"/", ApiPrefix + "/"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusFound {
				t.Errorf("expected 302, got %d", w.Code)
			}
			loc := w.Header().Get("Location")
			if loc != ApiPrefix+"/request" {
				t.Errorf("expected redirect to %s/request, got %s", ApiPrefix, loc)
			}
		})
	}
}

func TestRequest(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("request") {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if _, ok := body["env"]; !ok {
				t.Error("missing 'env' key")
			}
			if _, ok := body["headers"]; !ok {
				t.Error("missing 'headers' key")
			}
		})
	}
}

func TestHeaders(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("headers") {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("X-Test-Header", "test-value")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["x-test-header"] != "test-value" {
				t.Errorf("expected x-test-header=test-value, got %q", body["x-test-header"])
			}
		})
	}
}

func TestRedirect(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("redirect") {
		t.Run(path+" missing param", func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
		t.Run(path+" with param", func(t *testing.T) {
			req := httptest.NewRequest("GET", path+"?redirect_to=/somewhere", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusFound {
				t.Errorf("expected 302, got %d", w.Code)
			}
			if loc := w.Header().Get("Location"); loc != "/somewhere" {
				t.Errorf("expected Location=/somewhere, got %q", loc)
			}
		})
	}
}

func TestPing(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("ping") {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["status"] != "available" {
				t.Errorf("expected status=available, got %q", body["status"])
			}
		})
	}
}

func TestPrivatePing(t *testing.T) {
	mux := setupMux(t)
	for _, path := range []string{"/private/ping", ApiPrefix + "/private/ping"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["status"] != "available" {
				t.Errorf("expected status=available, got %q", body["status"])
			}
		})
	}
}

func TestUpload(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("upload") {
		t.Run(path, func(t *testing.T) {
			body := &strings.Builder{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.txt")
			io.WriteString(part, "hello world")
			writer.Close()

			req := httptest.NewRequest("POST", path, strings.NewReader(body.String()))
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if resp["status"] != "posted" {
				t.Errorf("expected status=posted, got %v", resp["status"])
			}
			if resp["upload_byte_size"] != float64(11) {
				t.Errorf("expected upload_byte_size=11, got %v", resp["upload_byte_size"])
			}
		})
	}
}

func TestIdentityMissing(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("identity") {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["errors"] == nil {
				t.Error("expected 'errors' key in response")
			}
		})
	}
}

func TestIdentityPresent(t *testing.T) {
	mux := setupMux(t)
	identity := base64.StdEncoding.EncodeToString([]byte(`{"identity": {}}`))
	for _, path := range routesFor("identity") {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("X-Rh-Identity", identity)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", w.Code)
			}
			if strings.TrimSpace(w.Body.String()) != `{"identity": {}}` {
				t.Errorf("unexpected body: %q", w.Body.String())
			}
		})
	}
}

func TestStatusOverride(t *testing.T) {
	mux := setupMux(t)
	req := httptest.NewRequest("GET", "/ping?status=401", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestSleepParam(t *testing.T) {
	mux := setupMux(t)
	req := httptest.NewRequest("GET", "/ping?sleep=1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSleepParamIgnoresFloat(t *testing.T) {
	mux := setupMux(t)
	start := time.Now()
	req := httptest.NewRequest("GET", "/ping?sleep=0.5", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if time.Since(start) >= 500*time.Millisecond {
		t.Error("expected float sleep param to be ignored")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebSocketNonUpgrade(t *testing.T) {
	mux := setupMux(t)
	for _, path := range routesFor("wss") {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if body["message"] != "not a websocket request" {
				t.Errorf("expected 'not a websocket request', got %q", body["message"])
			}
		})
	}
}

func TestVersionedRoute(t *testing.T) {
	mux := setupMux(t)
	req := httptest.NewRequest("GET", ApiPrefix+"/v1/ping", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "available" {
		t.Errorf("expected status=available, got %q", body["status"])
	}
}
