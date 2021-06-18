package wharf

import "github.com/itchio/lake/tlc"
import "io"

// AcceptListFilter only keeps files from an accept list
type AcceptListFilter struct {
	acceptList []string
}

func NewAcceptListFilter(acceptList []string) (*AcceptListFilter)  {
	return &AcceptListFilter{acceptList: acceptList};
}

func (e*AcceptListFilter) Filter(name string) tlc.FilterResult  {
	for _, pattern := range e.acceptList {
		if pattern == name {
			return tlc.FilterKeep
		}
	}

	return tlc.FilterIgnore
}

// NopWriteCloser turns a WriteCloser into a Writer, turning the Close() method into a no-op
type NopWriteCloser struct {
	writer io.Writer
}

func PreventClosing(w io.Writer) (io.Writer) {
	return &NopWriteCloser{writer: w}
}

func (n *NopWriteCloser) Write(buf []byte) (int, error) {
	return n.writer.Write(buf)
}

func (n *NopWriteCloser) Close() error {
	return nil
}
