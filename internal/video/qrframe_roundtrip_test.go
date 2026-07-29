package video

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ArqonAi/Pixelog/internal/qr"
	"github.com/ArqonAi/Pixelog/pkg/config"
)

// decodeFramePNG mirrors what decodeQRFromFrame does to an on-disk frame,
// minus the video step, so a failure here is purely generate → decode.
func decodeFramePNG(t *testing.T, m *Maker, path string) (string, error) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return "", err
	}
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return m.extractQRData(gray)
}

// TestQRFrameRoundTrip_NoVideo pins the generate → decode contract with the
// video encoder taken out of the picture.
//
// Extraction silently skips any frame it cannot decode and then aborts the
// whole file with "missing chunk index N", so a single undecodable frame
// costs the entire capsule. That failure was blamed on H.264 blur — there is
// a TRY_HARDER retry in extractQRData citing exactly that — but blur cannot
// explain a frame that fails to decode straight off disk.
func TestQRFrameRoundTrip_NoVideo(t *testing.T) {
	m := &Maker{}

	// Payload sizes spanning the range the converters actually emit. The
	// chunkers cap the MARSHALED frame at 2900 bytes, so the largest usable
	// payload is that minus the ~230-byte envelope.
	for _, payloadLen := range []int{256, 512, 1024, 1440, 1800, 2200, 2600} {
		t.Run(fmt.Sprintf("payload_%d", payloadLen), func(t *testing.T) {
			dir := t.TempDir()
			gen, err := qr.New(dir)
			if err != nil {
				t.Fatalf("qr.New: %v", err)
			}

			chunk := qr.Chunk{
				ID:         "abcd1234_0",
				Index:      0,
				Total:      1,
				Data:       strings.Repeat("A", payloadLen),
				SourceFile: "fixture.md",
				MimeType:   "text/markdown",
				Hash:       strings.Repeat("0", 64),
				CreatedAt:  time.Now().UTC().Truncate(time.Second),
			}

			marshaled, err := json.Marshal(chunk)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			paths, err := gen.GenerateFrames([]qr.Chunk{chunk})
			if err != nil {
				t.Fatalf("GenerateFrames rejected a %d-byte frame (%d bytes marshaled): %v",
					payloadLen, len(marshaled), err)
			}
			if len(paths) != 1 {
				t.Fatalf("expected 1 frame, got %d", len(paths))
			}

			got, err := decodeFramePNG(t, m, paths[0])
			if err != nil {
				t.Fatalf("frame with %d-byte payload (%d bytes marshaled) does not decode "+
					"off disk, with no video involved: %v", payloadLen, len(marshaled), err)
			}
			if got != string(marshaled) {
				t.Errorf("decoded payload differs from what was encoded (%d vs %d bytes)",
					len(got), len(marshaled))
			}
		})
	}
}

// TestDecodeReliability_Batch guards the decode strategy order.
//
// A per-frame failure rate that looks small is fatal here, because extraction
// needs every frame: at 5% a 28-frame capsule recovers about a quarter of the
// time. Reading frames with the default detector instead of PURE_BARCODE
// produced exactly that rate, so this asserts a clean sweep over a batch big
// enough that a few percent cannot pass unnoticed.
func TestDecodeReliability_Batch(t *testing.T) {
	m := &Maker{}
	dir := t.TempDir()
	gen, err := qr.New(dir)
	if err != nil {
		t.Fatalf("qr.New: %v", err)
	}

	// Deterministic pseudo-random content: the failures were content-
	// dependent, so a single fixed payload proves nothing.
	rng := rand.New(rand.NewSource(20260729))
	alphabet := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=")

	const batch = 60
	chunks := make([]qr.Chunk, 0, batch)
	for i := 0; i < batch; i++ {
		body := make([]byte, 1400+rng.Intn(600))
		for j := range body {
			body[j] = alphabet[rng.Intn(len(alphabet))]
		}
		chunks = append(chunks, qr.Chunk{
			ID:         fmt.Sprintf("abcd1234_%d", i),
			Index:      i,
			Total:      batch,
			Data:       string(body),
			SourceFile: "fixture-name.bin",
			MimeType:   "application/octet-stream",
			Hash:       strings.Repeat("f", 64),
			CreatedAt:  time.Unix(1750000000, 0).UTC(),
		})
	}

	paths, err := gen.GenerateFrames(chunks)
	if err != nil {
		t.Fatalf("GenerateFrames: %v", err)
	}

	var failed []int
	for i, p := range paths {
		text, err := decodeFramePNG(t, m, p)
		if err != nil {
			failed = append(failed, i)
			continue
		}
		want, err := json.Marshal(chunks[i])
		if err != nil {
			t.Fatal(err)
		}
		if text != string(want) {
			failed = append(failed, i)
		}
	}
	if len(failed) > 0 {
		t.Errorf("%d of %d frames failed to decode (indices %v); extraction needs all of them",
			len(failed), batch, failed)
	}
}

// TestQRFrameRoundTrip_ThroughVideo runs the real pipeline — generate frames,
// encode to .pixe, pull the frames back out — and reports per-frame whether
// the loss happens before or after the encoder.
//
// Extraction gives up with "missing chunk index N" as soon as one frame is
// undecodable, without saying which stage lost it. This test decodes both the
// frame written to disk and the frame recovered from the video, so a failure
// names the stage.
func TestQRFrameRoundTrip_ThroughVideo(t *testing.T) {
	// Sweep density: the streaming chunker emits ~1920-char base64 payloads,
	// so the encoder has to survive frames well past the ~1300 bytes an
	// earlier version of this test happened to use.
	for _, payloadLen := range []int{512, 1330, 1920, 2400, 2600} {
		t.Run(fmt.Sprintf("payload_%d", payloadLen), func(t *testing.T) {
			assertVideoRoundTrip(t, payloadLen)
		})
	}
}

func assertVideoRoundTrip(t *testing.T, payloadLen int) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH")
	}
	m, err := New()
	if err != nil {
		t.Skipf("video maker unavailable: %v", err)
	}

	dir := t.TempDir()
	frameDir := filepath.Join(dir, "frames")
	gen, err := qr.New(frameDir)
	if err != nil {
		t.Fatalf("qr.New: %v", err)
	}

	// 24 frames, each with distinct content at the requested density.
	const frames = 24
	chunks := make([]qr.Chunk, 0, frames)
	for i := 0; i < frames; i++ {
		prefix := fmt.Sprintf("frame-%03d:", i)
		body := strings.Repeat("Xy0123456789", payloadLen/12+1)
		data := (prefix + body)[:payloadLen]
		chunks = append(chunks, qr.Chunk{
			ID:         fmt.Sprintf("abcd1234_%d", i),
			Index:      i,
			Total:      frames,
			Data:       data,
			SourceFile: "fixture.md",
			MimeType:   "text/markdown",
			Hash:       strings.Repeat("a", 64),
			CreatedAt:  time.Now().UTC().Truncate(time.Second),
		})
	}

	paths, err := gen.GenerateFrames(chunks)
	if err != nil {
		t.Fatalf("GenerateFrames: %v", err)
	}

	// Every frame must decode straight off disk, else the encoder is not to
	// blame for anything that follows.
	preVideoBad := []int{}
	for i, p := range paths {
		if _, err := decodeFramePNG(t, m, p); err != nil {
			preVideoBad = append(preVideoBad, i)
		}
	}
	if len(preVideoBad) > 0 {
		t.Fatalf("frames %v do not decode before any video encoding", preVideoBad)
	}

	out := filepath.Join(dir, "out.pixe")
	cfg := &config.Config{ChunkSize: 2900, FrameRate: 2.0, Quality: 23, TempDir: dir, OutputDir: dir}
	if err := m.CreateVideo(paths, out, map[string]any{"total_chunks": frames}, cfg); err != nil {
		t.Fatalf("CreateVideo: %v", err)
	}

	extractDir := filepath.Join(dir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", out, "-vsync", "0",
		filepath.Join(extractDir, "frame_%05d.png"))
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg frame extraction: %v\n%s", err, combined)
	}

	got, err := filepath.Glob(filepath.Join(extractDir, "frame_*.png"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	if len(got) != frames {
		t.Errorf("video holds %d frames, wrote %d", len(got), frames)
	}

	postVideoBad := []int{}
	for i, p := range got {
		if _, err := decodeFramePNG(t, m, p); err != nil {
			postVideoBad = append(postVideoBad, i)
		}
	}
	if len(postVideoBad) > 0 {
		t.Errorf("frames %v decoded off disk but not after the video round trip "+
			"(%d of %d lost) — the encoder is destroying QR modules",
			postVideoBad, len(postVideoBad), frames)
	}
}
