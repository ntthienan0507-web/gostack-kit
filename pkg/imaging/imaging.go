// Package imaging provides image processing utilities: resize, convert, thumbnail.
// Designed for the upload pipeline: S3 event → Worker → process → upload variants back to S3.
//
// Supported input:  JPEG, PNG, GIF, BMP, TIFF
// Supported output: JPEG, PNG, WebP
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

// Format represents an output image format.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatWebP Format = "webp"
)

// ContentType returns the MIME type for the format.
func (f Format) ContentType() string {
	switch f {
	case FormatWebP:
		return "image/webp"
	case FormatPNG:
		return "image/png"
	default:
		return "image/jpeg"
	}
}

// Extension returns the file extension (with dot).
func (f Format) Extension() string {
	switch f {
	case FormatWebP:
		return ".webp"
	case FormatPNG:
		return ".png"
	default:
		return ".jpg"
	}
}

// Variant represents a processed image variant (e.g. thumbnail, webp conversion).
type Variant struct {
	Data        []byte // encoded image bytes
	Format      Format
	Width       int
	Height      int
	ContentType string
	Size        int // byte count
}

// ProcessOptions configures image processing.
type ProcessOptions struct {
	// MaxWidth/MaxHeight: resize to fit within bounds (maintains aspect ratio).
	// 0 = no resize.
	MaxWidth  int
	MaxHeight int

	// Quality: JPEG/WebP compression quality (1-100). Default 85.
	Quality int

	// Format: output format. Default = same as input.
	Format Format
}

func (o ProcessOptions) quality() int {
	if o.Quality <= 0 || o.Quality > 100 {
		return 85
	}
	return o.Quality
}

// Process decodes an image, optionally resizes, and encodes to the target format.
//
// Usage:
//
//	// Convert to WebP, max 800px wide
//	variant, err := imaging.Process(reader, imaging.ProcessOptions{
//	    MaxWidth: 800,
//	    Format:   imaging.FormatWebP,
//	    Quality:  80,
//	})
func Process(r io.Reader, opts ProcessOptions) (*Variant, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// Resize if needed (fit within bounds, preserve aspect ratio)
	if opts.MaxWidth > 0 || opts.MaxHeight > 0 {
		img = imaging.Fit(img, maxInt(opts.MaxWidth, 1), maxInt(opts.MaxHeight, 1), imaging.Lanczos)
	}

	bounds := img.Bounds()
	var buf bytes.Buffer

	switch opts.Format {
	case FormatWebP:
		if err := webp.Encode(&buf, img, &webp.Options{Quality: float32(opts.quality())}); err != nil {
			return nil, fmt.Errorf("encode webp: %w", err)
		}
	case FormatPNG:
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode png: %w", err)
		}
	default: // JPEG
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: opts.quality()}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
	}

	return &Variant{
		Data:        buf.Bytes(),
		Format:      opts.Format,
		Width:       bounds.Dx(),
		Height:      bounds.Dy(),
		ContentType: opts.Format.ContentType(),
		Size:        buf.Len(),
	}, nil
}

// Thumbnail creates a square thumbnail by cropping from center then resizing.
//
//	thumb, _ := imaging.Thumbnail(reader, 200, imaging.FormatWebP, 80)
func Thumbnail(r io.Reader, size int, format Format, quality int) (*Variant, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// CropCenter + Resize = square thumbnail
	thumb := imaging.Fill(img, size, size, imaging.Center, imaging.Lanczos)

	var buf bytes.Buffer
	q := quality
	if q <= 0 || q > 100 {
		q = 80
	}

	switch format {
	case FormatWebP:
		if err := webp.Encode(&buf, thumb, &webp.Options{Quality: float32(q)}); err != nil {
			return nil, fmt.Errorf("encode webp thumbnail: %w", err)
		}
	case FormatPNG:
		if err := png.Encode(&buf, thumb); err != nil {
			return nil, fmt.Errorf("encode png thumbnail: %w", err)
		}
	default:
		if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: q}); err != nil {
			return nil, fmt.Errorf("encode jpeg thumbnail: %w", err)
		}
	}

	return &Variant{
		Data:        buf.Bytes(),
		Format:      format,
		Width:       size,
		Height:      size,
		ContentType: format.ContentType(),
		Size:        buf.Len(),
	}, nil
}

// GenerateVariants processes an image into multiple variants in one call.
// Commonly used in upload pipeline to create all needed sizes at once.
//
// Example — avatar upload pipeline:
//
//	variants, err := imaging.GenerateVariants(reader, []imaging.ProcessOptions{
//	    {Format: imaging.FormatWebP, MaxWidth: 800, Quality: 85},    // main webp
//	    {Format: imaging.FormatWebP, MaxWidth: 200, Quality: 80},    // thumbnail
//	    {Format: imaging.FormatJPEG, MaxWidth: 800, Quality: 85},    // fallback jpeg
//	})
func GenerateVariants(r io.Reader, opts []ProcessOptions) ([]*Variant, error) {
	// Read all bytes so we can decode multiple times
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}

	variants := make([]*Variant, len(opts))
	for i, opt := range opts {
		v, err := Process(bytes.NewReader(data), opt)
		if err != nil {
			return nil, fmt.Errorf("variant %d: %w", i, err)
		}
		variants[i] = v
	}

	return variants, nil
}

// IsImage checks if a content type is a supported image format.
func IsImage(contentType string) bool {
	ct := strings.ToLower(contentType)
	return ct == "image/jpeg" || ct == "image/png" || ct == "image/gif" ||
		ct == "image/bmp" || ct == "image/tiff" || ct == "image/webp"
}

// IsImageKey checks if a file key/path has an image extension.
func IsImageKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".gif") ||
		strings.HasSuffix(lower, ".bmp") || strings.HasSuffix(lower, ".tiff") ||
		strings.HasSuffix(lower, ".webp")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
