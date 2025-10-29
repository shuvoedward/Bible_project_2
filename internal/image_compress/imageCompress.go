package image_compress

import (
	"fmt"
	"strings"

	"github.com/h2non/bimg"
)

type ImageProcessor struct {
	MaxWidth  int // 0 = no resize
	MaxHeight int // 0 = no resize
	Quality   int // 1-100, recommended 80-85 for good balance
}

func (ip *ImageProcessor) ProcessImageBuffer(buffer []byte, outputFormat string) ([]byte, error) {
	image := bimg.NewImage(buffer)

	size, err := image.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	options := bimg.Options{
		Quality: ip.Quality,
	}

	switch strings.ToLower(outputFormat) {
	case "webp":
		options.Type = bimg.WEBP
	case "jpg", "jpeg":
		options.Type = bimg.JPEG
	default:
		return nil, fmt.Errorf("unsupported output fortmatL %s", outputFormat)
	}

	if ip.MaxWidth > 0 || ip.MaxHeight > 0 {
		width, height := calculateResize(size.Width, size.Height, ip.MaxWidth, ip.MaxHeight)
		options.Width = width
		options.Height = height
	}

	processed, err := image.Process(options)
	if err != nil {
		return nil, fmt.Errorf("failed to process image: %w", err)
	}

	return processed, nil
}

func calculateResize(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
	if maxWidth == 0 && maxHeight == 0 {
		return origWidth, origHeight
	}

	if maxWidth == 0 {
		maxWidth = origWidth
	}

	if maxHeight == 0 {
		maxHeight = origHeight
	}

	// don't upscale
	if origWidth <= maxWidth && origHeight <= maxHeight {
		return origWidth, origHeight
	}

	ratio := float64(origWidth) / float64(origHeight)

	if origWidth > maxWidth {
		origWidth = maxWidth
		origHeight = int(float64(maxWidth) / ratio)
	}

	if origHeight > maxHeight {
		origHeight = maxHeight
		origWidth = int(float64(maxHeight) * ratio)
	}

	return origWidth, origHeight

}
