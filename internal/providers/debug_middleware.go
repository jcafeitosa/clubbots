package providers

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strings"
)

const redactedPlaceholder = "<REDACTED>"

var sensitiveHeaders = []string{
	"api-key",
	"x-api-key",
	"cookie",
	"set-cookie",
}

// DebugMiddleware logs HTTP requests and responses with sensitive header redaction.
// Enable via GOCLAW_DEBUG_HTTP=1 env var or by wrapping your HTTP client.
type DebugMiddleware struct {
	sensitiveHeaders []string
}

// NewDebugMiddleware creates a debug middleware with default sensitive headers.
func NewDebugMiddleware() *DebugMiddleware {
	return &DebugMiddleware{sensitiveHeaders: sensitiveHeaders}
}

// Wrap returns an http.RoundTripper that logs requests and responses.
func (m *DebugMiddleware) Wrap(next http.RoundTripper) http.RoundTripper {
	return &debugTransport{next: next, m: m}
}

type debugTransport struct {
	next http.RoundTripper
	m    *DebugMiddleware
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	redacted, err := t.m.redactRequest(req)
	if err == nil {
		if reqBytes, err := httputil.DumpRequest(redacted, true); err == nil {
			slog.Debug("http.request", "payload", string(reqBytes))
		}
	}

	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if respBytes, err := httputil.DumpResponse(resp, true); err == nil {
		slog.Debug("http.response", "payload", string(respBytes))
	}

	return resp, err
}

func (m *DebugMiddleware) redactRequest(req *http.Request) (*http.Request, error) {
	redactedHeaders := req.Header.Clone()

	if values := redactedHeaders.Values("Authorization"); len(values) > 0 {
		redactedHeaders.Del("Authorization")
		for _, value := range values {
			if authKind, _, ok := strings.Cut(value, " "); ok {
				redactedHeaders.Add("Authorization", authKind+" "+redactedPlaceholder)
			} else {
				redactedHeaders.Add("Authorization", redactedPlaceholder)
			}
		}
	}

	for _, header := range m.sensitiveHeaders {
		values := redactedHeaders.Values(header)
		if len(values) == 0 {
			continue
		}
		redactedHeaders.Del(header)
		for range values {
			redactedHeaders.Add(header, redactedPlaceholder)
		}
	}

	if reflect.DeepEqual(req.Header, redactedHeaders) {
		return req, nil
	}

	redacted := req.Clone(req.Context())
	redacted.Header = redactedHeaders
	var cloneErr error
	redacted.Body, req.Body, cloneErr = cloneRequestBody(req.Body)
	return redacted, cloneErr
}

func cloneRequestBody(b io.ReadCloser) (r1, r2 io.ReadCloser, err error) {
	if b == nil || b == http.NoBody {
		return http.NoBody, http.NoBody, nil
	}
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, err
	}
	if err = b.Close(); err != nil {
		return nil, b, err
	}
	return io.NopCloser(&buf), io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}
