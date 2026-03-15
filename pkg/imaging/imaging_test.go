package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testImage creates a simple 100x80 PNG image for testing.
func testImage(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestProcess_ToWebP(t *testing.T) {
	data := testImage(t)

	variant, err := Process(bytes.NewReader(data), ProcessOptions{
		Format:  FormatWebP,
		Quality: 80,
	})

	require.NoError(t, err)
	assert.Equal(t, FormatWebP, variant.Format)
	assert.Equal(t, "image/webp", variant.ContentType)
	assert.Equal(t, 100, variant.Width)
	assert.Equal(t, 80, variant.Height)
	assert.Greater(t, variant.Size, 0)
}

func TestProcess_Resize(t *testing.T) {
	data := testImage(t)

	variant, err := Process(bytes.NewReader(data), ProcessOptions{
		Format:   FormatJPEG,
		MaxWidth: 50,
		Quality:  85,
	})

	require.NoError(t, err)
	assert.LessOrEqual(t, variant.Width, 50)
	assert.Equal(t, "image/jpeg", variant.ContentType)
}

func TestProcess_PNG(t *testing.T) {
	data := testImage(t)

	variant, err := Process(bytes.NewReader(data), ProcessOptions{
		Format: FormatPNG,
	})

	require.NoError(t, err)
	assert.Equal(t, FormatPNG, variant.Format)
	assert.Equal(t, "image/png", variant.ContentType)
}

func TestThumbnail_Square(t *testing.T) {
	data := testImage(t) // 100x80

	thumb, err := Thumbnail(bytes.NewReader(data), 32, FormatWebP, 80)

	require.NoError(t, err)
	assert.Equal(t, 32, thumb.Width)
	assert.Equal(t, 32, thumb.Height)
	assert.Equal(t, FormatWebP, thumb.Format)
}

func TestGenerateVariants(t *testing.T) {
	data := testImage(t)

	variants, err := GenerateVariants(bytes.NewReader(data), []ProcessOptions{
		{Format: FormatWebP, MaxWidth: 800, Quality: 85},
		{Format: FormatWebP, MaxWidth: 50, Quality: 80},
		{Format: FormatJPEG, MaxWidth: 800, Quality: 85},
	})

	require.NoError(t, err)
	assert.Len(t, variants, 3)
	assert.Equal(t, FormatWebP, variants[0].Format)
	assert.Equal(t, FormatWebP, variants[1].Format)
	assert.LessOrEqual(t, variants[1].Width, 50)
	assert.Equal(t, FormatJPEG, variants[2].Format)
}

func TestIsImage(t *testing.T) {
	assert.True(t, IsImage("image/jpeg"))
	assert.True(t, IsImage("image/png"))
	assert.True(t, IsImage("image/gif"))
	assert.True(t, IsImage("image/webp"))
	assert.False(t, IsImage("application/pdf"))
	assert.False(t, IsImage("text/plain"))
}

func TestIsImageKey(t *testing.T) {
	assert.True(t, IsImageKey("avatar.jpg"))
	assert.True(t, IsImageKey("photo.PNG"))
	assert.True(t, IsImageKey("uploads/user/pic.webp"))
	assert.False(t, IsImageKey("document.pdf"))
	assert.False(t, IsImageKey("data.json"))
}

func TestFormat_ContentType(t *testing.T) {
	assert.Equal(t, "image/webp", FormatWebP.ContentType())
	assert.Equal(t, "image/png", FormatPNG.ContentType())
	assert.Equal(t, "image/jpeg", FormatJPEG.ContentType())
}

func TestFormat_Extension(t *testing.T) {
	assert.Equal(t, ".webp", FormatWebP.Extension())
	assert.Equal(t, ".png", FormatPNG.Extension())
	assert.Equal(t, ".jpg", FormatJPEG.Extension())
}

func TestProcess_DefaultQuality(t *testing.T) {
	data := testImage(t)

	// Quality 0 should default to 85
	variant, err := Process(bytes.NewReader(data), ProcessOptions{
		Format:  FormatJPEG,
		Quality: 0,
	})

	require.NoError(t, err)
	assert.Greater(t, variant.Size, 0)
}
