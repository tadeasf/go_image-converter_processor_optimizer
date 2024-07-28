package utils

import (
	"fmt"
	"log"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func ProcessFiles(files []string, resultsChan chan<- string, numWorkers int, format string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		processingStart := time.Now()
		processedFiles := processBatches(files, numWorkers, func(file string) error {
			err := ConvertAndOptimize([]string{file}, format)
			if err != nil {
				log.Printf("Failed to process file %s: %v", file, err)
				resultsChan <- fmt.Sprintf("Error processing %s: %v", file, err)
			} else {
				log.Printf("Processed file: %s", file)
				resultsChan <- fmt.Sprintf("Processed: %s", file)
			}
			return err
		})
		log.Printf("Processing completed in %v", time.Since(processingStart))

		close(resultsChan)

		end := time.Now()
		log.Printf("All files processed in %v", end.Sub(start))
		log.Printf("Successfully processed %d out of %d files", len(processedFiles), len(files))

		return nil
	}
}

func processBatches(files []string, numWorkers int, processFunc func(string) error) []string {
	var processedFiles []string
	var wg sync.WaitGroup
	var mu sync.Mutex

	fileChan := make(chan string, len(files))
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				err := processFunc(file)
				if err == nil {
					mu.Lock()
					processedFiles = append(processedFiles, file)
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	return processedFiles
}
