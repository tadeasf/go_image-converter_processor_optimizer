package main

import (
	"bufio"
	"fmt"
	"image-converter/utils"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strings"
	"time"

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
}

type tickMsg time.Time

func initialModel(files []string, numWorkers int, format string) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner:     s,
		files:       files,
		resultsChan: make(chan string, len(files)),
		numWorkers:  numWorkers,
		format:      format,
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
		utils.ProcessFiles(m.files, m.resultsChan, m.numWorkers, m.format),
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
				Value: 8,
				Usage: "Number of worker goroutines",
			},
		},
		Action: func(c *cli.Context) error {
			startTime := time.Now()

			// Interactive format selection
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Choose output format (webp/png/jpg): ")
			formatInput, _ := reader.ReadString('\n')
			format := strings.TrimSpace(strings.ToLower(formatInput))

			for format != "webp" && format != "png" && format != "jpg" {
				fmt.Print("Invalid format. Please choose webp, png, or jpg: ")
				formatInput, _ = reader.ReadString('\n')
				format = strings.TrimSpace(strings.ToLower(formatInput))
			}

			inputDir := c.Args().First()
			if inputDir == "" {
				inputDir = "." // Use current directory if no input is provided
			}
			files, err := utils.GetImageFiles(inputDir)
			if err != nil {
				return err
			}

			numWorkers := c.Int("workers")
			log.Printf("Starting processing with %d workers\n", numWorkers)

			m := initialModel(files, numWorkers, format)
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
