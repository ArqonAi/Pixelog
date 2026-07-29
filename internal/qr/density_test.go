package qr

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/makiuchi-d/gozxing"
	qrReader "github.com/makiuchi-d/gozxing/qrcode"
)

// decodeQR reads a frame back the way the extractor does.
func decodeQR(path string) (string, error) {
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

	// Mirror the production strategy order in internal/video.
	reader := qrReader.NewQRCodeReader()
	strategies := []map[gozxing.DecodeHintType]interface{}{
		{gozxing.DecodeHintType_PURE_BARCODE: true},
		{gozxing.DecodeHintType_PURE_BARCODE: true, gozxing.DecodeHintType_TRY_HARDER: true},
		nil,
		{gozxing.DecodeHintType_TRY_HARDER: true},
	}
	var lastErr error
	for _, hints := range strategies {
		bmp, err := gozxing.NewBinaryBitmapFromImage(gray)
		if err != nil {
			return "", err
		}
		res, derr := reader.Decode(bmp, hints)
		if derr == nil {
			return res.GetText(), nil
		}
		lastErr = derr
	}
	return "", lastErr
}

// TestFrameDensityCeiling pins that MaxFramePayload frames survive a
// generate → decode cycle, using content that forces QR byte mode.
//
// This is the constraint that made dense capsules unreadable. The chunkers
// sized frames against a 2900-byte marshaled limit, which lands on QR
// version 40 — and gozxing cannot reliably decode its own version-40 output,
// even straight off disk with no video in the path. Because extraction
// abandons the whole capsule on one undecodable frame, that made every
// sufficiently large .pixe a coin flip.
//
// Repetitive content is not a valid check here: it encodes in alphanumeric
// mode and reaches a lower version for the same byte count, which is why this
// went unnoticed.
func TestFrameDensityCeiling(t *testing.T) {
	dir := t.TempDir()

	// Mixed case + punctuation forces byte mode, the worst case, and is what
	// base64 payloads and real markdown actually look like.
	filler := "Xy09_aB/+zQ7{}\""

	// MaxFramePayload budgets the MARSHALED chunk, and JSON escaping inflates
	// the data unpredictably — the quotes in this filler add a byte each. So
	// find the ceiling by measuring, exactly as the production chunkers do,
	// rather than subtracting a fixed envelope.
	mkChunk := func(dataLen int) Chunk {
		return Chunk{
			ID:         "abcd1234_999999",
			Index:      999999,
			Total:      999999,
			Data:       strings.Repeat(filler, dataLen/len(filler)+2)[:dataLen],
			SourceFile: "a-reasonably-long-source-name.md",
			MimeType:   "application/octet-stream",
			Hash:       strings.Repeat("f", 64),
			CreatedAt:  time.Now().UTC().Truncate(time.Second),
		}
	}
	maxData := 0
	for n := 100; n <= MaxFramePayload; n += 10 {
		encoded, err := json.Marshal(mkChunk(n))
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) > MaxFramePayload {
			break
		}
		maxData = n
	}
	if maxData == 0 {
		t.Fatal("no data length fits a frame")
	}
	t.Logf("largest data length whose marshaled chunk fits %d bytes: %d", MaxFramePayload, maxData)

	for _, payloadLen := range []int{
		maxData / 4,
		maxData / 2,
		maxData - 200,
		maxData,
	} {
		t.Run(fmt.Sprintf("payload_%d", payloadLen), func(t *testing.T) {
			body := strings.Repeat(filler, payloadLen/len(filler)+2)[:payloadLen]
			chunk := Chunk{
				ID:         "abcd1234_999999",
				Index:      999999,
				Total:      999999,
				Data:       body,
				SourceFile: "a-reasonably-long-source-name.md",
				MimeType:   "application/octet-stream",
				Hash:       strings.Repeat("f", 64),
				CreatedAt:  time.Now().UTC().Truncate(time.Second),
			}

			marshaled, err := json.Marshal(chunk)
			if err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(dir, fmt.Sprintf("frame_%d.png", payloadLen))
			if err := encodeFrame(chunk, path); err != nil {
				t.Fatalf("encode rejected a %d-byte payload (%d marshaled): %v",
					payloadLen, len(marshaled), err)
			}

			got, err := decodeQR(path)
			if err != nil {
				t.Fatalf("a %d-byte payload (%d marshaled) does not decode off disk: %v",
					payloadLen, len(marshaled), err)
			}
			if got != string(marshaled) {
				t.Errorf("decoded text differs from encoded (%d vs %d bytes)",
					len(got), len(marshaled))
			}
		})
	}
}
