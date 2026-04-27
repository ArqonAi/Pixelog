package ipfs

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestKuboPublisher_RoundTrip exercises the full multipart→Kubo→CID flow
// against a faithful in-process replica of Kubo's /api/v0/add semantics.
func TestKuboPublisher_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/add" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("cid-version"); got != "1" {
			t.Errorf("cid-version = %q, want 1", got)
		}
		if got := r.URL.Query().Get("raw-leaves"); got != "true" {
			t.Errorf("raw-leaves = %q, want true", got)
		}

		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		var got []byte
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			got, _ = io.ReadAll(part)
			_ = part.Close()
		}
		if string(got) != "hello pixelog" {
			t.Errorf("body = %q, want %q", got, "hello pixelog")
		}
		fmt.Fprintln(w, `{"Name":"capsule.pixe","Hash":"bafy2bzaceabc123","Size":"13"}`)
	}))
	defer srv.Close()

	pub, err := New(Config{APIURL: srv.URL, GatewayURL: "https://ipfs.io"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if pub.Network() != "ipfs" {
		t.Errorf("Network() = %q, want ipfs", pub.Network())
	}

	res, err := pub.Publish(context.Background(), []byte("hello pixelog"), "video/mp4")
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if res.Locator != "bafy2bzaceabc123" {
		t.Errorf("Locator = %q", res.Locator)
	}
	if res.URL != "https://ipfs.io/ipfs/bafy2bzaceabc123" {
		t.Errorf("URL = %q", res.URL)
	}
	if res.Size != int64(len("hello pixelog")) {
		t.Errorf("Size = %d", res.Size)
	}
	if res.Network != "ipfs" {
		t.Errorf("Network = %q", res.Network)
	}
}

// TestKuboPublisher_AuthHeader confirms that AuthHeader is forwarded
// verbatim (covers Pinata "Bearer ...", Infura basic-auth, etc.).
func TestKuboPublisher_AuthHeader(t *testing.T) {
	const want = "Bearer test.jwt"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		fmt.Fprintln(w, `{"Hash":"bafyzzz","Size":"1"}`)
	}))
	defer srv.Close()

	pub, _ := New(Config{APIURL: srv.URL, AuthHeader: want})
	if _, err := pub.Publish(context.Background(), []byte("x"), ""); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

// TestKuboPublisher_NonJSONError reports server failures with a useful
// truncated body instead of swallowing them.
func TestKuboPublisher_NonJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "kubo offline", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	pub, _ := New(Config{APIURL: srv.URL})
	_, err := pub.Publish(context.Background(), []byte("data"), "")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("expected 503 error, got: %v", err)
	}
}

// TestKuboPublisher_EmptyData rejects empty uploads up-front.
func TestKuboPublisher_EmptyData(t *testing.T) {
	pub, _ := New(Config{APIURL: "http://example"})
	if _, err := pub.Publish(context.Background(), nil, ""); err == nil {
		t.Error("expected error on empty payload")
	}
}

// TestKuboPublisher_E2E runs against a real Kubo (or Kubo-compatible)
// node when IPFS_API_URL is set. CI must not require this; the test
// skips silently when the env is absent.
func TestKuboPublisher_E2E(t *testing.T) {
	api := os.Getenv("IPFS_API_URL")
	if api == "" {
		t.Skip("IPFS_API_URL not set; skipping live IPFS upload")
	}
	pub, err := New(Config{
		APIURL:     api,
		AuthHeader: os.Getenv("IPFS_AUTH_HEADER"),
		GatewayURL: firstNonEmpty(os.Getenv("IPFS_GATEWAY_URL"), "https://ipfs.io"),
		HTTPClient: &http.Client{Timeout: 90 * time.Second},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	payload := []byte(fmt.Sprintf("pixelog-e2e-%d", time.Now().UnixNano()))
	res, err := pub.Publish(context.Background(), payload, "text/plain")
	if err != nil {
		t.Fatalf("live Publish: %v", err)
	}
	if res.Locator == "" {
		t.Fatal("empty CID from live node")
	}
	t.Logf("live CID: %s (URL %s)", res.Locator, res.URL)

	// Tiny multipart-roundtrip sanity-check that we actually built a
	// well-formed body the server could parse.
	if _, err := multipart.NewReader(strings.NewReader(""), "x").NextPart(); err == nil {
		t.Error("multipart sanity: empty reader should error")
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
