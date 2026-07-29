package qr

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

type Generator struct {
	outputDir string
}

type Chunk struct {
	ID         string    `json:"id"`
	Index      int       `json:"index"`
	Total      int       `json:"total"`
	Data       string    `json:"data"`
	SourceFile string    `json:"source_file"`
	MimeType   string    `json:"mime_type"`
	Hash       string    `json:"hash"`
	Encrypted  bool      `json:"encrypted"`
	CreatedAt  time.Time `json:"created_at"`
}

func New(outputDir string) (*Generator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Generator{
		outputDir: outputDir,
	}, nil
}

// MaxFramePayload bounds the marshaled chunk JSON that may go into one QR
// frame. Chunkers must size their payloads so json.Marshal of the chunk stays
// within it.
//
// 2900 sits just under QR's byte-mode capacity at error-correction level L
// (2953), and version 40 decodes reliably now that frames are read with
// PURE_BARCODE — measured at 0 failures in 60 trials at every size from 452 to
// 2952 marshaled bytes. Before that change the same frames failed a few
// percent of the time and there was no safe size: the loss was caused by the
// decoder's perspective sampling, not by density.
//
// TestFrameDensityCeiling holds this to a size that survives the round trip.
const MaxFramePayload = 2900

// FrameSize is the pixel width and height of every QR frame.
//
// It must stay large enough that the densest frame still gets several pixels
// per QR module. gozxing scales the module matrix by an integer factor to fit
// the requested canvas, so the frame size sets the module resolution: a
// payload near the chunkers' 2900-byte limit needs QR version 40, which is
// 177 modules plus an 8-module quiet zone. At the previous 512px that left
// 512/185 = 2 pixels per module, and those frames intermittently failed to
// decode — losing one frame aborts the whole capsule with "missing chunk
// index N", which is why dense capsules were unreadable while sparse ones
// were fine.
//
// 1120 gives 6 pixels per module in the worst case and is a multiple of 16,
// so H.264 encodes it without padding the macroblock grid. Sparser frames get
// an even larger factor. Two-tone frames compress well, so the cost over 512px
// is far smaller than the area suggests.
const FrameSize = 1120

// encodeFrame writes one chunk as a QR PNG at the given path.
//
// The QR is encoded at its natural one-pixel-per-module size and scaled up
// here, rather than asking gozxing for a sized canvas.
//
// gozxing's own scaler produces undecodable output for some payloads: asked
// for any canvas larger than natural, certain QR versions come back as images
// its own reader cannot decode, while the same payload at natural size always
// decodes. The failures do not follow payload size — 1986 and 2626 marshaled
// bytes failed at every canvas from 512 to 2048 while 1666, 2200 and 2946
// succeeded — so no choice of canvas size avoids it. Scaling by an exact
// integer factor here keeps every module a crisp block and sidesteps the
// renderer entirely.
func encodeFrame(chunk Chunk, framePath string) error {
	chunkData, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to serialize chunk: %w", err)
	}
	if len(chunkData) > MaxFramePayload {
		return fmt.Errorf("marshaled chunk is %d bytes, over the %d-byte frame limit",
			len(chunkData), MaxFramePayload)
	}

	writer := qrcode.NewQRCodeWriter()
	hints := map[gozxing.EncodeHintType]interface{}{
		gozxing.EncodeHintType_ERROR_CORRECTION: "L",
	}
	// Width and height of 0 request the natural module matrix, no scaling.
	bitMatrix, err := writer.Encode(string(chunkData), gozxing.BarcodeFormat_QR_CODE, 0, 0, hints)
	if err != nil {
		return fmt.Errorf("failed to encode QR code (payload %d bytes): %w", len(chunkData), err)
	}

	img, err := scaleMatrix(bitMatrix)
	if err != nil {
		return err
	}

	file, err := os.Create(framePath)
	if err != nil {
		return fmt.Errorf("failed to create frame file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return fmt.Errorf("failed to save QR image: %w", err)
	}
	return nil
}

// scaleMatrix renders a module matrix onto a fixed FrameSize canvas, scaling by
// the largest integer factor that fits and centering the result on white.
//
// Every frame must be the same pixel size because they become video frames,
// and the scale must be an exact integer so module edges land on pixel
// boundaries.
func scaleMatrix(bm *gozxing.BitMatrix) (image.Image, error) {
	modules := bm.GetWidth()
	if modules <= 0 {
		return nil, fmt.Errorf("QR matrix has no modules")
	}
	if modules > FrameSize {
		return nil, fmt.Errorf("QR matrix is %d modules, larger than the %d-pixel frame", modules, FrameSize)
	}

	scale := FrameSize / modules
	used := modules * scale
	offset := (FrameSize - used) / 2

	img := image.NewGray(image.Rect(0, 0, FrameSize, FrameSize))
	// Quiet zone and background: the matrix from gozxing already carries a
	// quiet zone, and the centering padding extends it.
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	black := color.Gray{Y: 0x00}
	for my := 0; my < modules; my++ {
		for mx := 0; mx < modules; mx++ {
			if !bm.Get(mx, my) {
				continue
			}
			x0 := offset + mx*scale
			y0 := offset + my*scale
			for y := y0; y < y0+scale; y++ {
				for x := x0; x < x0+scale; x++ {
					img.SetGray(x, y, black)
				}
			}
		}
	}
	return img, nil
}

func (g *Generator) GenerateFrames(chunks []Chunk) ([]string, error) {
	var framePaths []string

	for i, chunk := range chunks {
		framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", i))
		if err := encodeFrame(chunk, framePath); err != nil {
			return nil, fmt.Errorf("chunk %d: %w", i, err)
		}
		framePaths = append(framePaths, framePath)
	}

	return framePaths, nil
}

func (g *Generator) GenerateFrame(chunk Chunk, frameNumber int) (string, error) {
	framePath := filepath.Join(g.outputDir, fmt.Sprintf("frame_%05d.png", frameNumber))
	if err := encodeFrame(chunk, framePath); err != nil {
		return "", err
	}
	return framePath, nil
}

func DecodeFrame(imagePath string) (*Chunk, error) {
	// Use gozxing for pure Go QR decoding
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("failed to create bitmap: %w", err)
	}

	reader := qrcode.NewQRCodeReader()
	hints := make(map[gozxing.DecodeHintType]interface{})
	hints[gozxing.DecodeHintType_PURE_BARCODE] = true
	result, err := reader.Decode(bmp, hints)
	if err != nil {
		return nil, fmt.Errorf("failed to decode QR code: %w", err)
	}

	var chunk Chunk
	if err := json.Unmarshal([]byte(result.GetText()), &chunk); err != nil {
		return nil, fmt.Errorf("failed to parse QR data: %w", err)
	}

	return &chunk, nil
}
