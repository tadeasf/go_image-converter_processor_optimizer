package utils

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/nickalie/go-webpbin"
)

func ConvertAndOptimize(inputPaths []string, format string) error {
	return convertAndOptimizeSingleFile(inputPaths[0], format)
}

func convertAndOptimizeSingleFile(inputPath, format string) error {
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
		// Use go-webpbin to decode WebP
		img, err = webpbin.Decode(file)
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
	outputPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + "." + format

	// Encode and optimize based on the chosen format
	switch format {
	case "jpg":
		err = imaging.Save(img, outputPath, imaging.JPEGQuality(85))
	case "png":
		err = imaging.Save(img, outputPath, imaging.PNGCompressionLevel(png.BestCompression))
	case "webp":
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer f.Close()

		return webpbin.Encode(f, img)
	}

	if err != nil {
		return fmt.Errorf("failed to save image %s: %v", outputPath, err)
	}

	// Remove the original file if it's different from the output
	if filepath.Ext(inputPath) != "."+format {
		if err := os.Remove(inputPath); err != nil {
			return fmt.Errorf("failed to remove original file %s: %v", inputPath, err)
		}
	}

	return nil
}

func GetImageFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isImageFile(strings.ToLower(filepath.Ext(path))) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func isImageFile(ext string) bool {
	supportedExtensions := []string{".webp", ".png", ".jpg", ".jpeg", ".gif", ".tiff", ".bmp"}
	for _, e := range supportedExtensions {
		if ext == e {
			return true
		}
	}
	return false
}
