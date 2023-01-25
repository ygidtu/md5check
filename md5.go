package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func absPath(path string) string {
	if inputPath == "" {
		if path, err := filepath.Abs(path); err != nil {
			log.Fatal(err)
		} else {
			return path
		}
	}

	return filepath.Join(inputPath, path)
}

type MD5 struct {
	Path  string
	MD5   string
	E     error
	Check bool
}

// newMd5 return new MD5
func newMD5(path, md5 string, check bool) *MD5 {
	return &MD5{Path: absPath(path), MD5: md5, Check: check}
}

// RelativePath returns relative path
func (m *MD5) RelativePath() string {
	res, err := filepath.Rel(inputPath, m.Path)
	if err != nil {
		log.Fatal(err)
	}
	return res
}

// String converts MD5 to string
func (m *MD5) String() string {
	path := m.RelativePath()
	if m.E != nil {
		return fmt.Sprintf("%s\t%v", path, m.E)
	}
	if m.Check {
		return fmt.Sprintf("%s\tok", path)
	}
	return fmt.Sprintf("%s\t%s", m.MD5, path)
}

// Hash generates a md5 hash from the given file or check the existed md5 string
func (m *MD5) Hash() {
	if _, err := os.Stat(m.Path); os.IsNotExist(err) {
		m.E = fmt.Errorf("not exist")
		return
	}

	f, err := os.Open(m.Path)
	if err != nil {
		m.E = fmt.Errorf("failed to open file: %v", err)
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Warnf("failed to close file: %v", err)
		}
	}()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		m.E = fmt.Errorf("failed at io.Copy: %v", err)
	} else if err == nil && m.MD5 == "" {
		m.MD5 = fmt.Sprintf("%x", h.Sum(nil))
	} else if err == nil && m.MD5 != fmt.Sprintf("%x", h.Sum(nil)) {
		m.E = fmt.Errorf("failed")
	}
}
