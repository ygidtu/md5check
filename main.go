package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	progressbar "github.com/schollz/progressbar/v3"
	goptions "github.com/voxelbrain/goptions"
)

// Create a new instance of the logger. You can have any number of instances.
var log = setLogger()

var (
	// version and build info
	buildStamp = "dev"
	gitHash    = "dev"
	goVersion  = "dev"
	version    = "dev"
	inputPath  string
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
func write(output string, wc chan string, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	writer := bufio.NewWriter(os.Stdout)
	if output != "" {
		// open output file
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
func LoopDirsFiles(path string, progress map[string]string) []*MD5 {
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
			if file.IsDir() {
				res = append(res, LoopDirsFiles(filepath.Join(path, file.Name()), progress)...)
			} else {
				p := filepath.Join(path, file.Name())
				md5_ := newMD5(p, "", false)
				if r, ok := progress[md5_.RelativePath()]; !ok {
					if p == ".git/logs/HEAD" {
						log.Infof("%v %v %v", md5_.RelativePath(), ok, r)
					}
					res = append(res, md5_)
				}
			}
		}
	}

	return res
}

func main() {
	options := struct {
		Input   string        `goptions:"-i, --input, description='The path to file or directory'"`
		Output  string        `goptions:"-o, --output, description='The path to output file, default save to stdout'"`
		Thread  int           `goptions:"-t, --thread, description='How many threads to use'"`
		Check   string        `goptions:"-c, --check, description='Check exist md5'"`
		Version bool          `goptions:"-v, --version, description='show version information'"`
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

	if options.Version {
		log.Infof("Current version: %s", version)
		log.Infof("Git Commit Hash: %s", gitHash)
		log.Infof("UTC Build Time : %s", buildStamp)
		log.Infof("Golang Version : %s", goVersion)
		os.Exit(0)
	}

	if options.Input != "" {
		if _, err := os.Stat(options.Input); os.IsNotExist(err) {
			log.Fatal("%s not exist", options.Input)
		}
	}
	inputPath = options.Input

	ic := make(chan *MD5)
	wc := make(chan string)
	var wg sync.WaitGroup

	progress := reloadProgress(options.Output, options.Check != "")
	if len(progress) > 0 {
		log.Infof("%d already finished", len(progress))
	}

	forCheck := make([]*MD5, 0)
	if options.Check == "" {
		log.Infof("Generate md5 for %s", options.Input)
		forCheck = append(forCheck, LoopDirsFiles(options.Input, progress)...)
	} else {
		log.Infof("Check md5 of %s", options.Check)
		for md5Str, path := range reloadProgress(options.Check, options.Check != "") {
			forCheck = append(forCheck, newMD5(absPath(path), md5Str, true))
		}
	}

	if len(forCheck) < 1 {
		log.Info("finished")
		os.Exit(0)
	}

	bar := progressbar.Default(int64(len(forCheck)))

	go write(options.Output, wc, &wg, bar)
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
