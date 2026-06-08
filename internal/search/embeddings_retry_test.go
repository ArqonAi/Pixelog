package search

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRetryableHTTPDo_Recovers429 confirms that a 429 from a flaky
// upstream is retried and the eventual 200 surfaces cleanly. The
// production failure mode this test guards is the one that hung
// the LoCoMo run: a single rate-limited request stalling the entire
// sequential QA pipeline forever.
func TestRetryableHTTPDo_Recovers429(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	mkReq := func() (*http.Request, error) {
		return http.NewRequest("POST", srv.URL, strings.NewReader("{}"))
	}
	resp, err := retryableHTTPDo(context.Background(), client, mkReq)
	if err != nil {
		t.Fatalf("retryableHTTPDo: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status=%d want 200", resp.StatusCode)
	}
	if calls != 3 {
		t.Errorf("calls=%d want 3 (two 429 + one 200)", calls)
	}
}

// TestRetryableHTTPDo_DoesNotRetry4xx confirms that 4xx errors other
// than 429 fail immediately — these are caller errors (bad auth,
// malformed body) that will fail identically on retry, so wasting
// API quota on them is a real footgun for production callers.
func TestRetryableHTTPDo_DoesNotRetry4xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	mkReq := func() (*http.Request, error) {
		return http.NewRequest("POST", srv.URL, strings.NewReader("{}"))
	}
	resp, err := retryableHTTPDo(context.Background(), client, mkReq)
	if err != nil {
		t.Fatalf("expected response not error on 401: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status=%d want 401", resp.StatusCode)
	}
	if calls != 1 {
		t.Errorf("calls=%d want 1 (no retry on 4xx)", calls)
	}
}

// TestRetryableHTTPDo_RespectsContextCancel confirms that an early
// context cancel during the backoff delay aborts the retry loop —
// the production failure mode is a request that's been retrying for
// minutes after the caller's per-QA timeout already fired.
func TestRetryableHTTPDo_RespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	mkReq := func() (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "POST", srv.URL, strings.NewReader("{}"))
	}
	_, err := retryableHTTPDo(ctx, client, mkReq)
	if err == nil {
		t.Fatal("expected error on ctx cancel")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "status") {
		t.Errorf("unexpected error: %v", err)
	}
}
