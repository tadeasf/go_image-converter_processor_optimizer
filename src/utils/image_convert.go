package utils

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"sync"

	"github.com/MaestroError/go-libheif"
	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

func ConvertAndOptimize(inputPaths []string, format, outputDir string, webpQuality int, fileNameMutex *sync.Mutex) error {
	for _, inputPath := range inputPaths {
		if err := convertAndOptimizeSingleFile(inputPath, format, outputDir, webpQuality, fileNameMutex); err != nil {
			return err
		}
	}
	return nil
}

func convertAndOptimizeSingleFile(inputPath, format, outputDir string, webpQuality int, fileNameMutex *sync.Mutex) error {
	var img image.Image
	var err error

	// Check the file extension
	ext := strings.ToLower(filepath.Ext(inputPath))

	switch ext {
	case ".webp":
		// Use github.com/chai2010/webp to decode WebP
		file, err := os.Open(inputPath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %v", inputPath, err)
		}
		defer file.Close()
		img, err = webp.Decode(file)
		if err != nil {
			return fmt.Errorf("failed to decode WebP image %s: %v", inputPath, err)
		}
	case ".heic", ".heif":
		// Use go-libheif to handle HEIC/HEIF
		if format == "webp" {
			// For WebP, we need to convert to JPEG first
			return convertHEICToWebP(inputPath, outputDir, webpQuality, fileNameMutex)
		}
		// For other formats, convert directly
		return convertHEICToFormat(inputPath, format, outputDir, fileNameMutex)
	default:
		// Use imaging for other formats
		img, err = imaging.Open(inputPath)
		if err != nil {
			return fmt.Errorf("failed to open image %s: %v", inputPath, err)
		}
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

	// Get a unique output path
	outputPath = getUniqueOutputPath(outputPath, fileNameMutex)

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

func convertHEICToFormat(inputPath, format, outputDir string, fileNameMutex *sync.Mutex) error {
	outputFileName := filepath.Base(inputPath)
	outputFileName = strings.TrimSuffix(outputFileName, filepath.Ext(outputFileName)) + "." + format
	outputPath := filepath.Join(outputDir, outputFileName)

	// Get a unique output path
	outputPath = getUniqueOutputPath(outputPath, fileNameMutex)

	switch format {
	case "jpg":
		return libheif.HeifToJpeg(inputPath, outputPath, 80)
	case "png":
		return libheif.HeifToPng(inputPath, outputPath)
	default:
		return fmt.Errorf("unsupported format for HEIC conversion: %s", format)
	}
}

func convertHEICToWebP(inputPath, outputDir string, webpQuality int, fileNameMutex *sync.Mutex) error {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "heic_conversion")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Convert HEIC to JPEG in the temp directory
	tempJPEGPath := filepath.Join(tempDir, "temp.jpg")
	err = libheif.HeifToJpeg(inputPath, tempJPEGPath, 100)
	if err != nil {
		return fmt.Errorf("failed to convert HEIC to JPEG: %v", err)
	}

	// Open the temporary JPEG file
	jpegFile, err := os.Open(tempJPEGPath)
	if err != nil {
		return fmt.Errorf("failed to open temporary JPEG file: %v", err)
	}
	defer jpegFile.Close()

	// Decode the JPEG
	img, err := jpeg.Decode(jpegFile)
	if err != nil {
		return fmt.Errorf("failed to decode temporary JPEG: %v", err)
	}

	// Resize the image
	img = imaging.Fit(img, 1440, 1440, imaging.Lanczos)

	// Prepare output path for WebP
	outputFileName := filepath.Base(inputPath)
	outputFileName = strings.TrimSuffix(outputFileName, filepath.Ext(outputFileName)) + ".webp"
	outputPath := filepath.Join(outputDir, outputFileName)

	// Get a unique output path
	outputPath = getUniqueOutputPath(outputPath, fileNameMutex)

	// Create the output file
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output WebP file: %v", err)
	}
	defer f.Close()

	// Encode to WebP
	options := &webp.Options{Lossless: false, Quality: float32(webpQuality)}
	if err := webp.Encode(f, img, options); err != nil {
		return fmt.Errorf("failed to encode WebP: %v", err)
	}

	return nil
}

// ConvertImage is now just an alias for convertAndOptimizeSingleFile
func ConvertImage(inputPath, format, outputDir string, webpQuality int, fileNameMutex *sync.Mutex) error {
	return convertAndOptimizeSingleFile(inputPath, format, outputDir, webpQuality, fileNameMutex)
}

func getUniqueOutputPath(outputPath string, fileNameMutex *sync.Mutex) string {
	dir := filepath.Dir(outputPath)
	ext := filepath.Ext(outputPath)
	name := strings.TrimSuffix(filepath.Base(outputPath), ext)

	counter := 1
	for {
		fileNameMutex.Lock()
		_, err := os.Stat(outputPath)
		fileNameMutex.Unlock()

		if os.IsNotExist(err) {
			return outputPath
		}
		outputPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}
}
