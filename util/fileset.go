package util

import (
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"sort"
)

// FileSet is a set of file paths relative to one basedir
type FileSet struct {
	basedir string
	files   map[string]bool
}

// NewFileSet creates an empty file set
func NewFileSet(basedir string) *FileSet {
	return &FileSet{basedir: basedir, files: map[string]bool{}}
}

// NewFileSetWith creates a file set with one initial file
func NewFileSetWith(basedir string, file string) *FileSet {
	result := NewFileSet(basedir)
	result.Add(file)
	return result
}

// Add adds a file, assumed to be relative to the FileSet's basedir
func (s *FileSet) Add(file string) {
	s.files[file] = true
}

// Len returns the number of files in the set
func (s *FileSet) Len() int {
	return len(s.files)
}

// TotalFileSize returns the total file size of all files, ignoring stat errors
func (s *FileSet) TotalFileSize() int64 {
	var result int64
	for file := range s.files {
		info, err := os.Stat(filepath.Join(s.basedir, file))
		if err != nil {
			log.Error().Str("file", file).Msg("Could not stat")
		}
		result += info.Size()
	}
	return result
}

// Walk runs a function on all files in the set
func (s *FileSet) Walk(f func(basedir string, file string)) {
	for file := range s.files {
		f(s.basedir, file)
	}
}

// Sorted returns a sorted list of paths
func (s *FileSet) Sorted() []string {
	result := []string{}
	for k := range s.files {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// Present returns true if a file is in the set
func (s *FileSet) Present(file string) bool {
	return s.files[file]
}

// BaseDir returns the set's base dir
func (s *FileSet) BaseDir() string {
	return s.basedir
}

// Merge merges two sets
func Merge(a *FileSet, b *FileSet) *FileSet {
	result := NewFileSet(a.basedir)
	b.Walk(func(_, file string) {
		result.Add(file)
	})
	return result
}

// Minus computes the difference between sets
func Minus(a *FileSet, b *FileSet) *FileSet {
	result := NewFileSet(a.basedir)
	a.Walk(func(_, file string) {
		if !b.Present(file) {
			result.Add(file)
		}
	})
	return result
}
