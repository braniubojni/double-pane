package imghash

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func photoImage(w, h, variant int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var c color.RGBA
			if variant == 0 {
				c = color.RGBA{
					uint8(40 + x*180/w),
					uint8(80 + y*140/h),
					uint8(200 - y*90/h),
					255,
				}
				if (x-w/3)*(x-w/3)+(y-h/3)*(y-h/3) < (w*w)/16 {
					c = color.RGBA{220, 60, 40, 255}
				}
			} else {
				if (x/(w/8+1)+y/(h/8+1))%2 == 0 {
					c = color.RGBA{10, 10, 10, 255}
				} else {
					c = color.RGBA{240, 240, 40, 255}
				}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func flagImage(w, h, variant int) *image.RGBA {
	return photoImage(w, h, variant)
}

func scale(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*b.Dx()/w
			sy := b.Min.Y + y*b.Dy()/h
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func jpegBuf(t *testing.T, img image.Image, q int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDHashResizedSameImage(t *testing.T) {
	src := photoImage(256, 256, 0)
	small := photoImage(80, 80, 0)
	a, err := Decode(bytes.NewReader(jpegBuf(t, src, 90)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Decode(bytes.NewReader(jpegBuf(t, small, 70)))
	if err != nil {
		t.Fatal(err)
	}
	sim := Similarity(DHash(a), DHash(b))
	if sim < 90 {
		t.Fatalf("resized same photo similarity %d, want >= 90", sim)
	}
}

func TestDHashDifferentImages(t *testing.T) {
	a := DHash(flagImage(128, 128, 0))
	b := DHash(flagImage(128, 128, 1))
	sim := Similarity(a, b)
	if sim >= 90 {
		t.Fatalf("unrelated photos similarity %d, want < 90", sim)
	}
}

func TestIsVisualName(t *testing.T) {
	if !IsVisualName("a.JPG") || IsVisualName("a.heic") || IsVisualName("a.txt") {
		t.Fatal("ext filter")
	}
	if !IsHEICName("b.HEIF") {
		t.Fatal("heic")
	}
}
