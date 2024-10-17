package main

import (
	"fmt"
	"image-converter/src/utils"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"time"

	"path/filepath"

	"strconv"

	"github.com/c-bata/go-prompt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"
)

type model struct {
	spinner     spinner.Model
	files       []string
	current     int
	done        bool
	resultsChan chan string
	numWorkers  int
	format      string
	outputDir   string
	webpQuality int
}

type tickMsg time.Time

func initialModel(files []string, numWorkers int, format string, outputDir string, webpQuality int) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:     s,
		files:       files,
		resultsChan: make(chan string, len(files)),
		numWorkers:  numWorkers,
		format:      format,
		outputDir:   outputDir,
		webpQuality: webpQuality,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		utils.ProcessFiles(m.files, m.resultsChan, m.numWorkers, m.format, m.outputDir, m.webpQuality),
		tickCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			close(m.resultsChan)
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tickMsg:
		select {
		case _, ok := <-m.resultsChan:
			if !ok {
				m.done = true
				return m, tea.Quit
			}
			m.current++
			return m, tea.Batch(m.spinner.Tick, tickCmd())
		default:
			return m, tickCmd()
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.done {
		return fmt.Sprintf("Done! Processed %d files.\n", len(m.files))
	}
	return fmt.Sprintf("%s Processing files...\n", m.spinner.View())
}

func main() {
	app := &cli.App{
		Name:  "image-optimizer",
		Usage: "Convert and optimize images",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "workers",
				Value: runtime.NumCPU(),
				Usage: "Number of worker goroutines",
			},
		},
		Action: func(c *cli.Context) error {
			startTime := time.Now()

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
				return err
			}

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

			m := initialModel(files, numWorkers, format, outputDir, webpQuality)
			p := tea.NewProgram(m, tea.WithAltScreen())

			// Open a log file
			logFile, err := os.Create("image_optimizer.log")
			if err != nil {
				return err
			}
			defer logFile.Close()

			// Set up logging
			log.SetOutput(logFile)

			// Start CPU profiling
			f, err := os.Create("cpu.prof")
			if err != nil {
				log.Fatal(err)
			}
			if err := pprof.StartCPUProfile(f); err != nil {
				log.Fatal("could not start CPU profile: ", err)
			}
			defer pprof.StopCPUProfile()

			// Start tracing
			traceFile, err := os.Create("trace.out")
			if err != nil {
				log.Fatal(err)
			}
			defer traceFile.Close()
			if err := trace.Start(traceFile); err != nil {
				log.Fatal("could not start tracing: ", err)
			}
			defer trace.Stop()

			if _, err := p.Run(); err != nil {
				return err
			}

			executionTime := time.Since(startTime)
			log.Printf("Total execution time: %v\n", executionTime)
			fmt.Printf("Total execution time: %v\n", executionTime)

			// Write memory profile
			f, err = os.Create("mem.prof")
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			runtime.GC() // get up-to-date statistics
			if err := pprof.WriteHeapProfile(f); err != nil {
				log.Fatal("could not write memory profile: ", err)
			}

			return nil
		},
	}

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
			suggestions = append(suggestions, prompt.Suggest{Text: filepath.Join(dir, file.Name())})
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
