package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RedHatInsights/http-test-services/internal"
)

// ApiPrefix is the path prefix for all routes. Configurable via the
// API_PREFIX environment variable; defaults to "/api/http-test-services".
var ApiPrefix = func() string {
	if v := os.Getenv(internal.EnvAPIPrefix); v != "" {
		return v
	}
	return "/api/http-test-services"
}()

// RegisterRoutes registers all HTTP route handlers on the given mux.
func RegisterRoutes(mux *http.ServeMux, docsDir string) {
	register := func(method, path string, handler http.HandlerFunc) {
		wrapped := withCommon(handler)
		if path == "/{$}" {
			mux.HandleFunc(method+" /{$}", wrapped)
			mux.HandleFunc(method+" "+ApiPrefix+"/{$}", wrapped)
			mux.HandleFunc(method+" "+ApiPrefix+"/{version}/{$}", wrapped)
		} else {
			mux.HandleFunc(method+" "+path, wrapped)
			mux.HandleFunc(method+" "+ApiPrefix+path, wrapped)
			mux.HandleFunc(method+" "+ApiPrefix+"/{version}"+path, wrapped)
		}
	}

	register("GET", "/{$}", redirectHandler)
	register("GET", "/request", requestHandler)
	register("GET", "/headers", headersHandler)
	register("GET", "/env", envHandler)
	register("GET", "/redirect", redirectToHandler)
	register("GET", "/ping", pingHandler)
	register("GET", "/private/ping", pingHandler)
	register("POST", "/upload", uploadHandler)
	register("GET", "/identity", identityHandler)
	register("GET", "/wss", WebSocketHandler)
	register("GET", "/sse", SSEHandler)

	openapiHandler := makeOpenapiHandler(docsDir)
	mux.HandleFunc("GET "+ApiPrefix+"/{version}/openapi.json", withCommon(openapiHandler))
}

// withCommon wraps a handler with request logging and sleep support.
func withCommon(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		if s := r.URL.Query().Get("sleep"); s != "" {
			if d, err := strconv.ParseFloat(s, 64); err == nil {
				time.Sleep(time.Duration(d * float64(time.Second)))
			}
		}
		next(w, r)
	}
}

// writeJSON writes a JSON response, respecting the ?status query param override.
func writeJSON(w http.ResponseWriter, r *http.Request, code int, data any) {
	if s := r.URL.Query().Get("status"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			code = n
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// writeJSONRaw writes a raw JSON byte slice response.
func writeJSONRaw(w http.ResponseWriter, r *http.Request, code int, raw []byte) {
	if s := r.URL.Query().Get("status"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			code = n
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(raw)
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	loc := ApiPrefix + "/request"
	w.Header().Set("Location", loc)
	w.WriteHeader(http.StatusFound)
	fmt.Fprintf(w, "Redirecting to %s\n", loc)
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"env":     requestEnv(r),
		"headers": sortedHeaders(r),
	})
}

func headersHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, sortedHeaders(r))
}

func envHandler(w http.ResponseWriter, r *http.Request) {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok {
			env[k] = v
		}
	}
	writeJSON(w, r, http.StatusOK, sortMapByKey(env))
}

func redirectToHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("redirect_to")
	if target == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Missing redirect_to query parameter")
		return
	}
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusFound)
	fmt.Fprintf(w, "Redirecting to %s\n", target)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "available"})
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var fileSize *int64
	file, _, err := r.FormFile("file")
	if err == nil {
		defer file.Close()
		n, _ := io.Copy(io.Discard, file)
		fileSize = &n
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"status":           "posted",
		"upload_byte_size": fileSize,
	})
}

func identityHandler(w http.ResponseWriter, r *http.Request) {
	identity := r.Header.Get("X-Rh-Identity")
	if identity == "" {
		writeJSON(w, r, http.StatusBadRequest, map[string]any{
			"errors": []map[string]any{
				{
					"detail": "No x-rh-identity header supplied in the request.",
					"status": 400,
				},
			},
		})
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(identity)
	if err != nil {
		writeJSON(w, r, http.StatusBadRequest, map[string]any{
			"errors": []map[string]any{
				{
					"detail": "Invalid base64 in x-rh-identity header.",
					"status": 400,
				},
			},
		})
		return
	}
	writeJSONRaw(w, r, http.StatusOK, decoded)
}

func makeOpenapiHandler(docsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(docsDir + "/openapi.json")
		if err != nil {
			log.Printf("Failed to read openapi.json: %v", err)
			http.Error(w, "openapi.json not found", http.StatusNotFound)
			return
		}
		writeJSONRaw(w, r, http.StatusOK, data)
	}
}

// sortedHeaders extracts HTTP headers lowercased and dash-separated, sorted by key.
func sortedHeaders(r *http.Request) orderedMap[string] {
	headers := make(map[string]string)
	for k, v := range r.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	return sortMapByKey(headers)
}

// requestEnv builds a map of request metadata similar to Rack env.
func requestEnv(r *http.Request) orderedMap[string] {
	env := map[string]string{
		"REQUEST_METHOD":  r.Method,
		"PATH_INFO":       r.URL.Path,
		"QUERY_STRING":    r.URL.RawQuery,
		"SERVER_PROTOCOL": r.Proto,
		"REMOTE_ADDR":     r.RemoteAddr,
		"REQUEST_URI":     r.RequestURI,
	}
	if r.Host != "" {
		env["HTTP_HOST"] = r.Host
	}
	if r.URL.Scheme != "" {
		env["rack.url_scheme"] = r.URL.Scheme
	}
	return sortMapByKey(env)
}

// sortMapByKey returns a json.Marshaler-friendly ordered map.
// Since Go maps don't preserve order, we use a slice of key-value pairs
// that marshals as a JSON object with sorted keys.
func sortMapByKey[V any](m map[string]V) orderedMap[V] {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entries := make([]mapEntry[V], len(keys))
	for i, k := range keys {
		entries[i] = mapEntry[V]{Key: k, Value: m[k]}
	}
	return orderedMap[V]{Entries: entries}
}

type mapEntry[V any] struct {
	Key   string
	Value V
}

type orderedMap[V any] struct {
	Entries []mapEntry[V]
}

func (o orderedMap[V]) MarshalJSON() ([]byte, error) {
	var buf []byte
	buf = append(buf, '{')
	for i, e := range o.Entries {
		if i > 0 {
			buf = append(buf, ',')
		}
		key, _ := json.Marshal(e.Key)
		val, err := json.Marshal(e.Value)
		if err != nil {
			return nil, err
		}
		buf = append(buf, key...)
		buf = append(buf, ':')
		buf = append(buf, val...)
	}
	buf = append(buf, '}')
	return buf, nil
}
