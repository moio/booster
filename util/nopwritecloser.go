package util

import "io"

// NopWriteCloser turns a WriteCloser into a Writer, turning the Close() method into a no-op
type NopWriteCloser struct {
	writer io.Writer
}

// PreventClosing returns a writer that does not Close()
func PreventClosing(w io.Writer) *NopWriteCloser {
	return &NopWriteCloser{writer: w}
}

// Write acts like io.Writer.Write
func (n *NopWriteCloser) Write(buf []byte) (int, error) {
	return n.writer.Write(buf)
}

// Close does nothing
func (n *NopWriteCloser) Close() error {
	return nil
}
