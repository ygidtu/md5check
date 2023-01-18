package main

import (
	"bufio"
	"crypto/md5"
	"fmt"
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
	buildStamp string
	gitHash    string
	goVersion  string
	version    string
)

type md5Res struct {
	Path  string
	MD5   string
	E     error
	Check bool
}

func new(path string) *md5Res {
	return &md5Res{Path: path}
}

func (m *md5Res) String() string {
	if m.E != nil {
		return fmt.Sprintf("%s\t%v", m.Path, m.E)
	}
	if m.Check {
		return fmt.Sprintf("%s\tok", m.Path)
	}
	return fmt.Sprintf("%s\t%s", m.MD5, m.Path)
}

func (m *md5Res) Hash() {
	if _, err := os.Stat(m.Path); os.IsNotExist(err) {
		m.E = fmt.Errorf("not exist")
		return
	}

	f, err := os.Open(m.Path)
	if err != nil {
		m.E = fmt.Errorf("failed to open file: %v", err)
		return
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		m.E = fmt.Errorf("failed at io.Copy: %v", err)
	} else if err == nil && m.MD5 == "" {
		m.MD5 = fmt.Sprintf("%x", h.Sum(nil))
	} else if err == nil && m.MD5 != fmt.Sprintf("%x", h.Sum(nil)) {
		m.E = fmt.Errorf("failed")
	}
}

func reloadProgress(path string) map[string]string {
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

		lines := pattern.Split(strings.ReplaceAll(line, "'", ""), -1)
		res[lines[0]] = lines[1]
	}
	return res
}

// worker in goroutines to extract md5 and check
func worker(wg *sync.WaitGroup, wc chan string, ic chan *md5Res) {
	defer wg.Done()
	for {
		file, ok := <-ic
		if !ok {
			break
		}
		file.Hash()
		// log.Infof("%v", file)
		wc <- file.String()
	}
}

func write(output string, wc chan string, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	// open output file
	f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to open %s: %s", output, err.Error())
		os.Exit(1)
	}

	writer := bufio.NewWriter(f)

	// clean writer and file
	defer wg.Done()
	defer writer.Flush()
	defer f.Close()
	defer bar.Finish()

	for {
		res, ok := <-wc
		if !ok {
			break
		}

		_, _ = writer.WriteString(res + "\n")

		writer.Flush()
		bar.Add(1)

		if bar.IsFinished() {
			break
		}
	}
}

func LoopDirsFiles(path string, progress map[string]string) []*md5Res {
	files, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	res := make([]*md5Res, 0)
	for _, file := range files {
		if file.IsDir() {
			res = append(res, LoopDirsFiles(filepath.Join(path, file.Name()), progress)...)
		} else {

			p := filepath.Join(path, file.Name())
			if _, ok := progress[p]; ok {
				continue
			}

			res = append(res, new(p))

		}
	}
	return res
}

func main() {
	options := struct {
		Input   string        `goptions:"-i, --input, description='The path to file or directory'"`
		Output  string        `goptions:"-o, --output, description='The path to output file'"`
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

	ic := make(chan *md5Res)
	wc := make(chan string)
	var wg sync.WaitGroup

	progress := reloadProgress(options.Output)
	if len(progress) > 0 {
		log.Infof("%d already finished", len(progress))
	}

	forCheck := make([]*md5Res, 0)
	if options.Check == "" {
		log.Infof("Generate md5 for %s", options.Input)
		forCheck = append(forCheck, LoopDirsFiles(options.Input, progress)...)
	} else {
		log.Infof("Check md5 of %s", options.Check)
		for md5Str, path := range reloadProgress(options.Check) {
			forCheck = append(forCheck, &md5Res{Path: filepath.Join(options.Input, path), MD5: md5Str, Check: true})
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
