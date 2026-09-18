package imghash

import (
	"context"
	"image"
	"image/color"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// MaxBytes is the visual/OCR decode cap. Exact SHA-256 still considers larger files.
const MaxBytes int64 = 50 << 20

// IsVisualName reports jpg/jpeg/png/webp/gif. HEIC is not decoded in V2.1.
func IsVisualName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}

// IsHEICName is true for HEIC/HEIF names we skip (no decoder).
func IsHEICName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".heic", ".heif":
		return true
	}
	return false
}

// Hamming is the bit distance between two 64-bit dHashes.
func Hamming(a, b uint64) int { return bits.OnesCount64(a ^ b) }

// Similarity is (64 - hamming) / 64 * 100, rounded down.
func Similarity(a, b uint64) int {
	return (64 - Hamming(a, b)) * 100 / 64
}

// ClampPct keeps the UI slider range 70–100.
func ClampPct(n int) int {
	if n < 70 {
		return 70
	}
	if n > 100 {
		return 100
	}
	return n
}

// DHash is a 64-bit difference hash: 9×8 grayscale, adjacent horizontal bits.
func DHash(src image.Image) uint64 {
	const w, h = 9, 8
	if src == nil {
		return 0
	}
	gray := resampleGray(src, w, h)
	if gray == nil {
		return 0
	}
	var hash uint64
	for y := 0; y < h; y++ {
		row := gray[y*w:]
		for x := 0; x < 8; x++ {
			if row[x] < row[x+1] {
				hash |= 1 << uint(y*8+x)
			}
		}
	}
	return hash
}

func resampleGray(src image.Image, dw, dh int) []uint8 {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw < 1 || sh < 1 || dw < 1 || dh < 1 {
		return nil
	}
	out := make([]uint8, dw*dh)
	for y := 0; y < dh; y++ {
		y0 := b.Min.Y + y*sh/dh
		y1 := b.Min.Y + (y+1)*sh/dh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < dw; x++ {
			x0 := b.Min.X + x*sw/dw
			x1 := b.Min.X + (x+1)*sw/dw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum, n uint32
			for sy := y0; sy < y1 && sy < b.Max.Y; sy++ {
				for sx := x0; sx < x1 && sx < b.Max.X; sx++ {
					sum += uint32(toGray(src.At(sx, sy)))
					n++
				}
			}
			if n > 0 {
				out[y*dw+x] = uint8(sum / n)
			}
		}
	}
	return out
}

func toGray(c color.Color) uint8 {
	r, _, _, _ := color.GrayModel.Convert(c).RGBA()
	return uint8(r >> 8)
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// Decode reads a jpeg/png/gif/webp image.
func Decode(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	return img, err
}

// HashReader decodes r and returns its dHash. r is closed if it is a Closer.
func HashReader(ctx context.Context, r io.Reader) (uint64, error) {
	if c, ok := r.(io.Closer); ok {
		defer func() { _ = c.Close() }()
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	img, err := Decode(ctxReader{ctx: ctx, r: r})
	if err != nil {
		return 0, err
	}
	return DHash(img), ctx.Err()
}

// HashFile opens path, decodes, and returns a 64-bit dHash.
func HashFile(ctx context.Context, path string) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	return HashReader(ctx, f)
}
