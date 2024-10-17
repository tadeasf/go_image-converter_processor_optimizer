package utils

import (
	"io/fs"
	"log"
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

type ProcessResult struct {
	SuccessCount int
	FailCount    int
	FailedFiles  []string
}

func ProcessFiles(files []string, resultsChan chan<- ProcessResult, numWorkers int, format, outputDir string, webpQuality int, verbose bool, noLimit bool) tea.Cmd {
	return func() tea.Msg {
		log.Printf("ProcessFiles function started with %d files", len(files))
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, numWorkers)
		result := ProcessResult{}
		var resultMutex sync.Mutex

		for _, file := range files {
			wg.Add(1)
			go func(file string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				log.Printf("Processing file: %s", file)
				err := ConvertImage(file, format, outputDir, webpQuality, verbose, noLimit)
				resultMutex.Lock()
				if err != nil {
					log.Printf("Error processing %s: %v", file, err)
					result.FailCount++
					result.FailedFiles = append(result.FailedFiles, file)
				} else {
					log.Printf("Successfully processed %s", file)
					result.SuccessCount++
				}
				resultMutex.Unlock()
			}(file)
		}

		log.Printf("Waiting for all goroutines to complete")
		wg.Wait()
		log.Printf("All goroutines completed. Sending result: %+v", result)
		resultsChan <- result
		close(resultsChan)
		log.Printf("ResultsChan closed")
		return result
	}
}
