package main

import (
	"fmt"
	"image-converter/src/utils"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"path/filepath"

	"strconv"

	"io"

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
	"gopkg.in/natefinch/lumberjack.v2"
)

type model struct {
	spinner     spinner.Model
	files       []string
	done        bool
	resultsChan chan utils.ProcessResult
	numWorkers  int
	format      string
	outputDir   string
	webpQuality int
	verbose     bool
	noLimit     bool
	result      utils.ProcessResult
}

type tickMsg time.Time

func initialModel(files []string, numWorkers int, format string, outputDir string, webpQuality int, verbose bool, noLimit bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:     s,
		files:       files,
		resultsChan: make(chan utils.ProcessResult, len(files)),
		numWorkers:  numWorkers,
		format:      format,
		outputDir:   outputDir,
		webpQuality: webpQuality,
		verbose:     verbose,
		noLimit:     noLimit,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	log.Printf("Model Init called")
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			log.Printf("Starting file processing")
			go utils.ProcessFiles(m.files, m.resultsChan, m.numWorkers, m.format, m.outputDir, m.webpQuality, m.verbose, m.noLimit)()
			log.Printf("File processing started in background")
			return tickMsg(time.Now())
		},
		tickCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("Update called with message type: %T", msg)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			log.Printf("Quit command received")
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tickMsg:
		select {
		case result, ok := <-m.resultsChan:
			if !ok {
				log.Printf("resultsChan closed")
				m.done = true
				return m, tea.Quit
			}
			log.Printf("Received result from channel: %+v", result)
			m.result = result
			m.done = true
			return m, tea.Quit
		default:
			return m, tickCmd()
		}
	case utils.ProcessResult:
		log.Printf("Received ProcessResult: %+v", msg)
		m.result = msg
		m.done = true
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.done {
		totalFiles := m.result.SuccessCount + m.result.FailCount
		successRate := float64(m.result.SuccessCount) / float64(totalFiles) * 100

		output := fmt.Sprintf("Done! Processed %d files.\n", totalFiles)
		output += fmt.Sprintf("Conversion success rate: %d/%d (%.2f%%)\n", m.result.SuccessCount, totalFiles, successRate)
		output += fmt.Sprintf("Number of failed conversions: %d\n", m.result.FailCount)

		configDir, _ := os.UserConfigDir()
		logFile := filepath.Join(configDir, "go-image-converter", "image_optimizer.log")
		output += fmt.Sprintf("Error log file location: %s\n", logFile)

		if m.result.FailCount > 0 {
			errorDir := filepath.Join(m.outputDir, "errors")
			output += fmt.Sprintf("Failed conversions copied to: %s\n", errorDir)
		}

		return output
	}
	return fmt.Sprintf("%s Processing files...\n", m.spinner.View())
}

func main() {
	log.Printf("Application started")
	app := &cli.App{
		Name:  "image-optimizer",
		Usage: "Convert and optimize images",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "workers",
				Value: runtime.NumCPU(),
				Usage: "Number of worker goroutines",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Value: false,
				Usage: "Enable verbose output",
			},
			&cli.BoolFlag{
				Name:  "nolimit",
				Value: false,
				Usage: "Disable image resizing",
			},
		},
		Action: func(c *cli.Context) error {
			log.Printf("Starting application action")
			startTime := time.Now()

			// Set up logging
			configDir, err := os.UserConfigDir()
			if err != nil {
				log.Fatal("Error getting user config directory:", err)
			}
			logDir := filepath.Join(configDir, "go-image-converter")
			if err := os.MkdirAll(logDir, 0755); err != nil {
				log.Fatal("Error creating log directory:", err)
			}
			logFile := filepath.Join(logDir, "image_optimizer.log")

			log.SetOutput(&lumberjack.Logger{
				Filename:   logFile,
				MaxSize:    10, // megabytes
				MaxBackups: 3,
				MaxAge:     28, // days
			})
			log.Printf("Logging initialized")

			log.Printf("Log file created at: %s", logFile)

			verbose := c.Bool("verbose")

			// Interactive input path selection
			var inputDir string
			if c.Args().First() == "" {
				inputDir = prompt.Input("Enter input directory: ", pathCompleter)
			} else {
				inputDir = c.Args().First()
			}

			// Interactive format selection using go-prompt
			format := prompt.Input(
				"Choose output format: ",
				formatCompleter,
				prompt.OptionCompletionWordSeparator(" "),
				prompt.OptionPreviewSuggestionTextColor(prompt.Yellow),
				prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
				prompt.OptionSuggestionBGColor(prompt.DarkGray),
			)

			// Prompt for recursive search
			recursive := promptYesNo("Do you want to search recursively?")

			// Prompt for WebP quality if the format is webp
			var webpQuality int
			if format == "webp" {
				webpQuality = promptForWebPQuality()
			}

			files, err := utils.GetImageFiles(inputDir, recursive)
			if err != nil {
				log.Printf("Error getting image files: %v", err)
				return err
			}
			log.Printf("Found %d files to process", len(files))

			numWorkers := c.Int("workers")
			if numWorkers <= 0 {
				numWorkers = runtime.NumCPU()
				if numWorkers <= 0 {
					numWorkers = 1
				}
			}

			// Create output directory
			outputDir := filepath.Join(inputDir, format)
			err = os.MkdirAll(outputDir, 0755)
			if err != nil {
				return err
			}

			noLimit := c.Bool("nolimit")

			log.Printf("Initializing model")
			m := initialModel(files, numWorkers, format, outputDir, webpQuality, verbose, noLimit)

			log.Printf("Starting tea program")
			p := tea.NewProgram(m, tea.WithAltScreen())

			log.Printf("Running tea program")
			if _, err := p.Run(); err != nil {
				log.Printf("Error running tea program: %v", err)
				logError(err, verbose)
				return err
			}

			log.Printf("Tea program completed")

			// Copy failed files to errors directory
			if m.result.FailCount > 0 {
				errorDir := filepath.Join(outputDir, "errors")
				if err := os.MkdirAll(errorDir, 0755); err != nil {
					return fmt.Errorf("failed to create error directory: %v", err)
				}

				for _, failedFile := range m.result.FailedFiles {
					destPath := filepath.Join(errorDir, filepath.Base(failedFile))
					if err := copyFile(failedFile, destPath); err != nil {
						logError(fmt.Errorf("failed to copy %s to error directory: %v", failedFile, err), verbose)
					}
				}
			}

			executionTime := time.Since(startTime)
			logInfo(fmt.Sprintf("Total execution time: %v", executionTime), verbose)

			// Write memory profile
			// f, err = os.Create("mem.prof")
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// defer f.Close()
			// runtime.GC() // get up-to-date statistics
			// if err := pprof.WriteHeapProfile(f); err != nil {
			// 	log.Fatal("could not write memory profile: ", err)
			// }

			return nil
		},
	}

	log.Printf("Running app")
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func pathCompleter(d prompt.Document) []prompt.Suggest {
	path := d.Text
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	files, _ := os.ReadDir(dir)
	var suggestions []prompt.Suggest
	for _, file := range files {
		if strings.HasPrefix(file.Name(), base) {
			suggestion := filepath.Join(dir, file.Name())
			if file.IsDir() {
				suggestion += "/"
			}
			suggestions = append(suggestions, prompt.Suggest{Text: suggestion})
		}
	}
	return suggestions
}

func formatCompleter(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "webp", Description: "WebP format"},
		{Text: "png", Description: "PNG format"},
		{Text: "jpg", Description: "JPEG format"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func promptYesNo(question string) bool {
	for {
		answer := prompt.Input(question+" (y/N): ", func(d prompt.Document) []prompt.Suggest {
			s := []prompt.Suggest{
				{Text: "y", Description: "Yes"},
				{Text: "n", Description: "No"},
			}
			return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
		})
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "y" {
			return true
		} else if answer == "n" || answer == "" {
			return false
		}
		fmt.Println("Please enter 'y' for yes or 'n' for no.")
	}
}

func promptForWebPQuality() int {
	qualities := []prompt.Suggest{
		{Text: "100", Description: "Lossless"},
		{Text: "97", Description: "Very high quality"},
		{Text: "95", Description: "High quality"},
		{Text: "90", Description: "Good quality"},
		{Text: "85", Description: "Balanced quality"},
		{Text: "80", Description: "Moderate compression"},
		{Text: "75", Description: "Higher compression"},
		{Text: "60", Description: "Maximum compression"},
	}

	result := prompt.Input(
		"Choose WebP quality: ",
		func(d prompt.Document) []prompt.Suggest {
			return prompt.FilterHasPrefix(qualities, d.GetWordBeforeCursor(), true)
		},
		prompt.OptionCompletionWordSeparator(" "),
		prompt.OptionPreviewSuggestionTextColor(prompt.Yellow),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
	)

	quality, _ := strconv.Atoi(result)
	return quality
}

func logError(err error, verbose bool) {
	log.Printf("ERROR: %v", err)
	if verbose {
		fmt.Printf("ERROR: %v\n", err)
	}
}

func logInfo(msg string, verbose bool) {
	if verbose {
		fmt.Println(msg)
	}
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
