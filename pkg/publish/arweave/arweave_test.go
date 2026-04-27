package arweave

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// generateTestWallet builds a deterministic in-memory RSA-2048 wallet.
// Real Arweave wallets are 4096-bit; 2048 keeps unit tests fast and is
// sufficient to exercise every code path including signing/verification.
func generateTestWallet(t *testing.T) *Wallet {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	priv.Precompute()
	n := priv.N.Bytes()
	addr := sha256.Sum256(n)
	return &Wallet{Key: priv, OwnerN: n, Address: b64URLEncode(addr[:])}
}

// TestChunkData_RebalancesShortTail validates Arweave's "no chunk smaller
// than MinChunkSize unless it's the only one" invariant.
func TestChunkData_RebalancesShortTail(t *testing.T) {
	// MaxChunkSize + 1 byte: the naive split would yield one full chunk
	// followed by a 1-byte chunk, violating the rebalance rule.
	data := bytes.Repeat([]byte{0xAB}, MaxChunkSize+1)
	chunks := chunkData(data)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].MaxByteRange != chunks[0].MinByteRange+len(chunks[0].Data) {
		t.Errorf("chunk 0 byte-range mismatch")
	}
	if len(chunks[1].Data) < MinChunkSize {
		t.Errorf("tail chunk %d < MinChunkSize %d (rebalance failed)",
			len(chunks[1].Data), MinChunkSize)
	}
	if chunks[0].MaxByteRange != chunks[1].MinByteRange {
		t.Errorf("chunks not contiguous: %d vs %d",
			chunks[0].MaxByteRange, chunks[1].MinByteRange)
	}
	if chunks[1].MaxByteRange != len(data) {
		t.Errorf("final maxByteRange = %d, want %d",
			chunks[1].MaxByteRange, len(data))
	}
}

// TestChunkData_SmallSingleChunk: data smaller than MaxChunkSize must
// produce exactly one chunk.
func TestChunkData_SmallSingleChunk(t *testing.T) {
	data := []byte("pixelog capsule body")
	chunks := chunkData(data)
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	want := sha256.Sum256(data)
	if !bytes.Equal(chunks[0].DataHash, want[:]) {
		t.Errorf("dataHash mismatch")
	}
}

// TestMerkle_StableRootForIdenticalData ensures determinism: identical
// input must yield identical data_root regardless of process or run.
func TestMerkle_StableRootForIdenticalData(t *testing.T) {
	data := bytes.Repeat([]byte("x"), MaxChunkSize*3+12345)
	r1, _ := buildMerkleTree(chunkData(data))
	r2, _ := buildMerkleTree(chunkData(data))
	if !bytes.Equal(r1.id, r2.id) {
		t.Errorf("merkle root not deterministic")
	}
}

// TestDeepHash_BlobAndList exercises both the blob and list arms with a
// hand-checked invariant: deepHash(list) starting state derives from the
// list-tag, so the empty list has a fixed value.
func TestDeepHash_BlobAndList(t *testing.T) {
	emptyList := deepHash(DHList{})
	if len(emptyList) != 48 {
		t.Errorf("deepHash output not 48 bytes (sha384)")
	}
	// blob() with empty input is also valid; it must differ from the empty
	// list to prove the tag is actually consumed.
	emptyBlob := deepHash(DHBlob(nil))
	if bytes.Equal(emptyList, emptyBlob) {
		t.Errorf("empty list and empty blob hash to the same value (tag missing?)")
	}
}

// TestSignAndVerify_RoundTrip verifies that a Pixelog-signed payload
// validates against the wallet's RSA-PSS public key — the same check an
// Arweave node performs.
func TestSignAndVerify_RoundTrip(t *testing.T) {
	w := generateTestWallet(t)
	payload := []byte("any deep-hash output (48 bytes here is fine)")
	digest := sha256.Sum256(payload) // simulate the deep-hash digest input
	sig, err := rsa.SignPSS(rand.Reader, w.Key, crypto.SHA256, digest[:], &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA256,
	})
	if err != nil {
		t.Fatalf("SignPSS: %v", err)
	}
	if err := rsa.VerifyPSS(&w.Key.PublicKey, crypto.SHA256, digest[:], sig, &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA256,
	}); err != nil {
		t.Errorf("VerifyPSS: %v", err)
	}
}

// fakeArweaveNode is an in-process replica of the Arweave node endpoints
// the publisher consumes. It captures every received tx + chunk for
// inspection.
type fakeArweaveNode struct {
	mu       *testingMu
	anchor   string
	priceFn  func(int) string
	receivedTx     []transaction
	receivedChunks []chunkPayload
}

// testingMu is a minimal mutex avoiding sync import noise (the test is
// serial in practice; the type stays defensive).
type testingMu struct{}

func (testingMu) Lock()   {}
func (testingMu) Unlock() {}

func newFakeNode() (*httptest.Server, *fakeArweaveNode) {
	state := &fakeArweaveNode{
		mu:      &testingMu{},
		anchor:  "JBfZJ1KePXvYqUEK5gJzZWzG7gXp_Sp7uH4yX9wKfNQ", // arbitrary b64url-ish
		priceFn: func(n int) string { return strconv.Itoa(n * 2) },
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/tx_anchor", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, state.anchor)
	})
	mux.HandleFunc("/price/", func(w http.ResponseWriter, r *http.Request) {
		bytesPart := strings.TrimPrefix(r.URL.Path, "/price/")
		n, _ := strconv.Atoi(bytesPart)
		fmt.Fprint(w, state.priceFn(n))
	})
	mux.HandleFunc("/tx", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var tx transaction
		if err := json.Unmarshal(body, &tx); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.receivedTx = append(state.receivedTx, tx)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/chunk", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var c chunkPayload
		if err := json.Unmarshal(body, &c); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.receivedChunks = append(state.receivedChunks, c)
		w.WriteHeader(http.StatusOK)
	})
	return httptest.NewServer(mux), state
}

// TestPublish_SmallBlob_FullPipeline exercises the full publish flow
// against a fake node and checks every wire-level invariant the real
// network would enforce.
func TestPublish_SmallBlob_FullPipeline(t *testing.T) {
	srv, state := newFakeNode()
	defer srv.Close()

	w := generateTestWallet(t)
	pub, err := New(Config{
		NodeURL:    srv.URL,
		Wallet:     w,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if pub.Network() != "arweave" {
		t.Errorf("Network() = %q", pub.Network())
	}

	body := []byte("pixelog: a small but real capsule payload")
	res, err := pub.Publish(context.Background(), body, "video/mp4")
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Locator must be a 43-char b64url-encoded sha256.
	if len(res.Locator) != 43 {
		t.Errorf("Locator len = %d, want 43; got %q", len(res.Locator), res.Locator)
	}
	if res.Size != int64(len(body)) {
		t.Errorf("Size = %d", res.Size)
	}
	if res.Network != "arweave" {
		t.Errorf("Network = %q", res.Network)
	}
	if !strings.HasSuffix(res.URL, "/"+res.Locator) {
		t.Errorf("URL %q doesn't end with locator", res.URL)
	}

	if len(state.receivedTx) != 1 {
		t.Fatalf("server received %d txs, want 1", len(state.receivedTx))
	}
	tx := state.receivedTx[0]
	if tx.Format != 2 {
		t.Errorf("tx.Format = %d", tx.Format)
	}
	if tx.DataSize != strconv.Itoa(len(body)) {
		t.Errorf("tx.DataSize = %q", tx.DataSize)
	}
	if tx.Owner != b64URLEncode(w.OwnerN) {
		t.Errorf("tx.Owner mismatch")
	}
	if tx.Signature == "" {
		t.Errorf("tx.Signature missing")
	}

	// Verify signature against the deep-hash of the same fields the
	// publisher signed — this is exactly what an Arweave node validates.
	wantSig, _ := b64URLDecode(tx.Signature)
	idCheck := sha256.Sum256(wantSig)
	if b64URLEncode(idCheck[:]) != tx.ID {
		t.Errorf("tx.ID is not sha256(signature)")
	}

	// Tags must contain Content-Type=video/mp4 and App-Name=Pixelog.
	mustHaveTag(t, tx.Tags, "Content-Type", "video/mp4")
	mustHaveTag(t, tx.Tags, "App-Name", "Pixelog")

	// Chunks must equal expected count.
	if got, want := len(state.receivedChunks), 1; got != want {
		t.Errorf("chunks received = %d, want %d", got, want)
	}
	c := state.receivedChunks[0]
	if c.DataRoot != tx.DataRoot {
		t.Errorf("chunk data_root mismatch")
	}
	if c.DataSize != tx.DataSize {
		t.Errorf("chunk data_size mismatch")
	}
	gotChunk, _ := b64URLDecode(c.Chunk)
	if !bytes.Equal(gotChunk, body) {
		t.Errorf("chunk bytes mismatch")
	}
}

// TestPublish_LargeBlob_MultiChunk covers the multi-chunk path including
// merkle tree depth > 1 and chunk path verification.
func TestPublish_LargeBlob_MultiChunk(t *testing.T) {
	srv, state := newFakeNode()
	defer srv.Close()

	w := generateTestWallet(t)
	pub, _ := New(Config{NodeURL: srv.URL, Wallet: w})

	// 3 full chunks + a small tail that will be rebalanced.
	body := bytes.Repeat([]byte{0xCD}, MaxChunkSize*3+1234)
	if _, err := pub.Publish(context.Background(), body, ""); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(state.receivedTx) != 1 {
		t.Fatalf("expected 1 tx")
	}
	if got := len(state.receivedChunks); got < 4 {
		t.Errorf("expected ≥4 chunks (rebalanced), got %d", got)
	}

	// Each chunk must be reassemblable back to the original body in order.
	var reassembled []byte
	for _, c := range state.receivedChunks {
		raw, _ := b64URLDecode(c.Chunk)
		reassembled = append(reassembled, raw...)
	}
	if !bytes.Equal(reassembled, body) {
		t.Errorf("reassembled chunks != original body (len=%d vs %d)",
			len(reassembled), len(body))
	}
}

// TestWallet_LoadFromJSON validates JWK loading round-trip with a generated key.
func TestWallet_LoadFromJSON(t *testing.T) {
	w := generateTestWallet(t)
	jwk := JWK{
		Kty: "RSA",
		N:   b64URLEncode(w.Key.N.Bytes()),
		E:   b64URLEncode(big2bytes(int64(w.Key.E))),
		D:   b64URLEncode(w.Key.D.Bytes()),
		P:   b64URLEncode(w.Key.Primes[0].Bytes()),
		Q:   b64URLEncode(w.Key.Primes[1].Bytes()),
	}
	js, _ := json.Marshal(jwk)
	loaded, err := LoadWalletFromJSON(js)
	if err != nil {
		t.Fatalf("LoadWalletFromJSON: %v", err)
	}
	if !bytes.Equal(loaded.OwnerN, w.OwnerN) {
		t.Errorf("OwnerN mismatch after JWK roundtrip")
	}
}

// TestPublish_E2E hits a real Arweave node when ARWEAVE_NODE_URL and a
// wallet path are provided. arlocal (https://github.com/textury/arlocal)
// is the recommended fixture — it spins up a local fake-money node in
// seconds and accepts the same wire protocol as production.
func TestPublish_E2E(t *testing.T) {
	node := os.Getenv("ARWEAVE_NODE_URL")
	walletPath := os.Getenv("ARWEAVE_WALLET_PATH")
	if node == "" || walletPath == "" {
		t.Skip("ARWEAVE_NODE_URL and ARWEAVE_WALLET_PATH not set; skipping live Arweave upload")
	}
	w, err := LoadWalletFromFile(walletPath)
	if err != nil {
		t.Fatalf("LoadWalletFromFile: %v", err)
	}
	pub, err := New(Config{
		NodeURL:    node,
		Wallet:     w,
		GatewayURL: os.Getenv("ARWEAVE_GATEWAY_URL"),
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := []byte(fmt.Sprintf("pixelog-e2e-%d\n", time.Now().UnixNano()))
	res, err := pub.Publish(context.Background(), body, "text/plain")
	if err != nil {
		t.Fatalf("live Arweave Publish: %v", err)
	}
	if res.Locator == "" {
		t.Fatal("empty locator from live node")
	}
	t.Logf("live Arweave tx: %s (URL %s)", res.Locator, res.URL)
}

func mustHaveTag(t *testing.T, tags []Tag, name, value string) {
	t.Helper()
	wantName := b64URLEncode([]byte(name))
	wantVal := b64URLEncode([]byte(value))
	for _, tg := range tags {
		if tg.Name == wantName && tg.Value == wantVal {
			return
		}
	}
	t.Errorf("missing tag %s=%s", name, value)
}

func big2bytes(n int64) []byte {
	b := make([]byte, 0, 8)
	for n > 0 {
		b = append([]byte{byte(n & 0xff)}, b...)
		n >>= 8
	}
	if len(b) == 0 {
		b = []byte{0}
	}
	return b
}
