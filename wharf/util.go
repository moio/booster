package wharf

import (
	"github.com/itchio/lake/tlc"
	"path/filepath"
)
import "io"

// AcceptListFilter only keeps files from an accept list
type AcceptListFilter struct {
	basedir  string
	accepted map[string]bool
}

func NewAcceptListFilter(basedir string, accepted map[string]bool) *AcceptListFilter {
	return &AcceptListFilter{basedir: basedir, accepted: accepted}
}

func (e *AcceptListFilter) Filter(name string) tlc.FilterResult {
	relpath, _ := filepath.Rel(e.basedir, name)
	if e.accepted[relpath] {
		return tlc.FilterKeep
	}
	return tlc.FilterIgnore
}

// NopWriteCloser turns a WriteCloser into a Writer, turning the Close() method into a no-op
type NopWriteCloser struct {
	writer io.Writer
}

func PreventClosing(w io.Writer) *NopWriteCloser {
	return &NopWriteCloser{writer: w}
}

func (n *NopWriteCloser) Write(buf []byte) (int, error) {
	return n.writer.Write(buf)
}

func (n *NopWriteCloser) Close() error {
	return nil
}
