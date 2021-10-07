package util

import (
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"
	"sort"
)

// FileSet is a set of file paths relative to one basedir
type FileSet struct {
	files map[string]bool
}

// NewFileSet creates an empty file set
func NewFileSet() *FileSet {
	return &FileSet{files: map[string]bool{}}
}

// NewFileSetWith creates a file set with one initial file
func NewFileSetWith(file string) *FileSet {
	result := NewFileSet()
	result.Add(file)
	return result
}

// Add adds a file, assumed to be relative to the FileSet's basedir
func (s *FileSet) Add(file string) {
	s.files[file] = true
}

// Walk runs a function on all files in the set
func (s *FileSet) Walk(f func(file string)) {
	for file := range s.files {
		f(file)
	}
}

// Len returns the number of files in the set
func (s *FileSet) Len() int {
	return len(s.files)
}

// TotalFileSize returns the total file size of all files, ignoring stat errors
func (s *FileSet) TotalFileSize() int64 {
	var result int64
	s.Walk(func(f string) {
		info, err := os.Stat(f)
		if err != nil {
			log.Error().Str("file", f).Msg("Could not stat")
		}
		result += info.Size()
	})
	return result
}

// Relative returns a set relative to a directory
func (s *FileSet) Relative(basedir string) (*FileSet, error) {
	result := NewFileSet()
	for f := range s.files {
		rel, err := filepath.Rel(basedir, f)
		if err != nil {
			return nil, err
		}
		result.Add(rel)
	}
	return result, nil
}

// Sorted returns all files, sorted
func (s *FileSet) Sorted() []string {
	result := []string{}
	s.Walk(func(f string) {
		result = append(result, f)
	})
	sort.Strings(result)
	return result
}

// Present returns true if a file is in the set
func (s *FileSet) Present(file string) bool {
	return s.files[file]
}

// Merge merges two sets
func Merge(a *FileSet, b *FileSet) *FileSet {
	result := NewFileSet()
	b.Walk(func(file string) {
		result.Add(file)
	})
	return result
}

// Minus computes the difference between sets
func Minus(a *FileSet, b *FileSet) *FileSet {
	result := NewFileSet()
	a.Walk(func(file string) {
		if !b.Present(file) {
			result.Add(file)
		}
	})
	return result
}
