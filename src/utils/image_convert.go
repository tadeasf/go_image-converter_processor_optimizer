package utils

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

func ConvertAndOptimize(inputPaths []string, format, outputDir string, webpQuality int) error {
	for _, inputPath := range inputPaths {
		if err := convertAndOptimizeSingleFile(inputPath, format, outputDir, webpQuality); err != nil {
			return err
		}
	}
	return nil
}

func convertAndOptimizeSingleFile(inputPath, format, outputDir string, webpQuality int) error {
	var img image.Image
	var err error

	// Open the input file
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", inputPath, err)
	}
	defer file.Close()

	// Check if the input file is WebP
	if strings.ToLower(filepath.Ext(inputPath)) == ".webp" {
		// Use github.com/chai2010/webp to decode WebP
		img, err = webp.Decode(file)
	} else {
		// Use imaging for other formats
		img, err = imaging.Decode(file)
	}

	if err != nil {
		return fmt.Errorf("failed to decode image %s: %v", inputPath, err)
	}

	// Resize the image (only downscale) to long side max 1440
	img = imaging.Fit(img, 1440, 1440, imaging.Lanczos)

	// Prepare output path
	outputFileName := filepath.Base(inputPath)
	outputFileName = strings.TrimSuffix(outputFileName, filepath.Ext(outputFileName)) + "." + format
	outputPath := filepath.Join(outputDir, outputFileName)

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Encode and optimize based on the chosen format
	switch format {
	case "jpg":
		err = imaging.Save(img, outputPath, imaging.JPEGQuality(80))
	case "png":
		err = imaging.Save(img, outputPath, imaging.PNGCompressionLevel(png.BestCompression))
	case "webp":
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer f.Close()

		options := &webp.Options{Lossless: false, Quality: float32(webpQuality)}
		if err := webp.Encode(f, img, options); err != nil {
			return fmt.Errorf("failed to encode WebP: %v", err)
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to save image %s: %v", outputPath, err)
	}

	return nil
}

// ConvertImage is now just an alias for convertAndOptimizeSingleFile
func ConvertImage(inputPath, format, outputDir string, webpQuality int) error {
	return convertAndOptimizeSingleFile(inputPath, format, outputDir, webpQuality)
}
