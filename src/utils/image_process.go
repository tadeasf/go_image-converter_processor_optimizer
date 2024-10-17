package utils

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

func GetImageFiles(root string, recursive bool) ([]string, error) {
	var files []string
	validExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
		".heif": true, ".heic": true, ".gif": true, ".tiff": true, ".bmp": true,
	}

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if validExtensions[ext] {
				files = append(files, path)
			}
		} else if !recursive && path != root {
			return fs.SkipDir
		}
		return nil
	}

	err := filepath.WalkDir(root, walkFunc)
	return files, err
}

func ProcessFiles(files []string, resultsChan chan<- string, numWorkers int, format, outputDir string, webpQuality int) tea.Cmd {
	return func() tea.Msg {
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, numWorkers)

		for _, file := range files {
			wg.Add(1)
			go func(file string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				err := ConvertImage(file, format, outputDir, webpQuality)
				if err != nil {
					fmt.Printf("Error processing %s: %v\n", file, err)
				}
				resultsChan <- file
			}(file)
		}

		wg.Wait()
		close(resultsChan)
		return nil
	}
}
