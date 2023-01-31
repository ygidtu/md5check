package main

import (
	"bufio"
	"fmt"
	"github.com/k0kubun/go-ansi"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/voxelbrain/goptions"
)

// Create a new instance of the logger. You can have any number of instances.
var (
	// version and build info
	buildStamp = "dev"
	gitHash    = "dev"
	goVersion  = "dev"
	version    = "dev"
	inputPath  string
	log        *zap.SugaredLogger
)

// reloadProgress is used to resume the progress
func reloadProgress(path string, check bool) map[string]string {
	res := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return res
	}

	pattern := regexp.MustCompile(`\s+`)

	r := bufio.NewReader(f)

	for {
		line, err := r.ReadString('\n')

		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}

		lines := pattern.Split(strings.ReplaceAll(strings.TrimSpace(line), "'", ""), -1)
		if len(lines) > 1 {
			if check {
				res[lines[0]] = lines[1]
			} else {
				res[lines[1]] = lines[0]
			}
		}
	}
	return res
}

// worker in goroutines to extract md5 and check
func worker(wg *sync.WaitGroup, wc chan string, ic chan *MD5) {
	defer wg.Done()
	for {
		file, ok := <-ic
		if !ok {
			break
		}
		file.Hash()
		wc <- file.String()
	}
}

// write saves the md5 and check results
func write(output string, wc chan string, wg *sync.WaitGroup, bar *progressbar.ProgressBar, resume bool) {
	writer := bufio.NewWriter(os.Stdout)
	if output != "" {
		// open output file
		mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		if resume {
			mode = os.O_CREATE | os.O_WRONLY | os.O_APPEND
		}
		f, err := os.OpenFile(output, mode, 0644)
		if err != nil {
			log.Fatalf("failed to open %s: %s", output, err.Error())
			os.Exit(1)
		}

		writer = bufio.NewWriter(f)

		defer func() {
			if err := writer.Flush(); err != nil {
				log.Fatalf("failed to flush: %v", err)
			}
			if err := f.Close(); err != nil {
				log.Fatalf("failed to close %s: %s", output, err.Error())
			}
		}()
	}

	// clean writer and file
	defer wg.Done()

	defer func() {
		if err := bar.Finish(); err != nil {
			log.Fatal(err)
		}
	}()

	for {
		res, ok := <-wc
		if !ok {
			break
		}

		_, _ = writer.WriteString(res + "\n")
		_ = writer.Flush()
		_ = bar.Add(1)

		if bar.IsFinished() {
			break
		}
	}
}

// LoopDirsFiles returns a list of filepath under the given directory
func LoopDirsFiles(path string, progress map[string]string, hidden bool) []*MD5 {
	res := make([]*MD5, 0)
	if stat, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatal(err)
	} else if !stat.IsDir() {
		md5_ := newMD5(path, "", false)
		if _, ok := progress[md5_.RelativePath()]; !ok {
			res = append(res, md5_)
		}
	} else {
		files, err := os.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if !hidden && strings.HasPrefix(file.Name(), ".") {
				continue
			}

			if file.IsDir() {
				res = append(res, LoopDirsFiles(filepath.Join(path, file.Name()), progress, hidden)...)
			} else {
				md5_ := newMD5(filepath.Join(path, file.Name()), "", false)
				if _, ok := progress[md5_.RelativePath()]; !ok {
					res = append(res, md5_)
				}
			}
		}
	}

	return res
}

// ProgressBar customizes the progressbar, enable ANSICodes to avoid blank print in terminal
func ProgressBar(size int) *progressbar.ProgressBar {
	return progressbar.NewOptions(size,
		progressbar.OptionSetWriter(ansi.NewAnsiStderr()),
		progressbar.OptionUseANSICodes(true), // avoid progressbar downsize error
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionFullWidth(),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionOnCompletion(func() {
			if _, err := fmt.Fprint(os.Stderr, "\n"); err != nil {
				log.Fatal(err)
			}
		}),
		progressbar.OptionSetWidth(10))
}

func main() {
	options := struct {
		Input   string        `goptions:"-i, --input, description='The path to file or directory'"`
		Output  string        `goptions:"-o, --output, description='The path to output file, default save to stdout'"`
		Thread  int           `goptions:"-t, --thread, description='How many threads to use'"`
		Check   string        `goptions:"-c, --check, description='Check exist md5'"`
		Resume  bool          `goptions:"-r, --resume, description='Resume and skip finished files'"`
		Hidden  bool          `goptions:"-f, --hidden, description='Calculate md5 to hidden files'"`
		Debug   bool          `goptions:"--debug, description='Show debug information'"`
		Version bool          `goptions:"-v, --version, description='Show version information'"`
		Help    goptions.Help `goptions:"-h, --help, description='Show this help'"`
	}{
		Input:  "./",
		Thread: 4,
	}
	goptions.ParseAndFail(&options)

	if len(os.Args) <= 1 {
		goptions.PrintHelp()
		os.Exit(0)
	}

	log = setLogger(options.Debug)
	log.Debugf("Enable debug level logging")

	if options.Version {
		log.Infof("Current version: %s", version)
		log.Infof("Git Commit Hash: %s", gitHash)
		log.Infof("UTC Build Time : %s", buildStamp)
		log.Infof("Golang Version : %s", goVersion)
		return
	}
	defer log.Info("finished")

	if options.Input != "" {
		if _, err := os.Stat(options.Input); os.IsNotExist(err) {
			log.Fatal("%s not exist", options.Input)
		}
	}
	inputPath = options.Input

	ic := make(chan *MD5)
	wc := make(chan string)
	var wg sync.WaitGroup

	progress := make(map[string]string)
	if options.Resume {
		progress = reloadProgress(options.Output, options.Check != "")
		if len(progress) > 0 {
			log.Infof("%d already finished", len(progress))
		}
	}

	forCheck := make([]*MD5, 0)
	if options.Check == "" {
		log.Infof("Generate md5 for %s", options.Input)
		forCheck = append(forCheck, LoopDirsFiles(options.Input, progress, options.Hidden)...)
	} else {
		log.Infof("Check md5 of %s", options.Check)
		for md5Str, path := range reloadProgress(options.Check, options.Check != "") {
			forCheck = append(forCheck, newMD5(absPath(path), md5Str, true))
		}
	}

	if len(forCheck) < 1 {
		return
	}

	bar := ProgressBar(len(forCheck))
	go write(options.Output, wc, &wg, bar, options.Resume)
	wg.Add(1)

	for i := 0; i < options.Thread; i++ {
		go worker(&wg, wc, ic)
		wg.Add(1)
	}

	for _, data := range forCheck {
		ic <- data
	}
	close(ic)
	wg.Wait()
	close(wc)
}
