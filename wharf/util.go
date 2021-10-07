package wharf

import (
	"github.com/itchio/lake/tlc"
	"github.com/moio/booster/util"
	"path/filepath"
)

// FileSetFilter only keeps files from an util.FileSet
type FileSetFilter struct {
	set *util.FileSet
}

// NewFileSetFilter returns a new filter based on a set
func NewFileSetFilter(set *util.FileSet) *FileSetFilter {
	return &FileSetFilter{set: set}
}

// Filter implements tlc.FilterFunc
func (e *FileSetFilter) Filter(name string) tlc.FilterResult {
	rel, _ := filepath.Rel(e.set.BaseDir(), name)
	if e.set.Present(rel) {
		return tlc.FilterKeep
	}
	return tlc.FilterIgnore
}
