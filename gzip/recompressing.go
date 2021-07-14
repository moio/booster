package gzip

import (
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"hash"
	"io"
)

// RecompressibilityReader is a gzip reader which checks, while reading, whether the file can be
// later be recompressed resulting in the exact same binary ("transparent recompressibility").
// This is currently the case if the gzip stream was created with Go's implementation
// and standard compression level
type RecompressibilityReader struct {
	reader *gzip.Reader
	writer *gzip.Writer
	tee2   io.Reader
	h1     hash.Hash
	h2     hash.Hash
}

// NewRecompressibilityReader returns a gzip.RecompressibilityReader which also checks whether its contents can be recompressed transparently
func NewRecompressibilityReader(r io.Reader) (*RecompressibilityReader, error) {
	// r -> tee1 -> h1
	//       |----> reader -> tee2 -> writer -> h2
	//                         |----> caller

	h1 := sha512.New()
	tee1 := io.TeeReader(r, h1)
	reader, err := gzip.NewReader(tee1)
	if err != nil {
		return nil, err
	}

	h2 := sha512.New()
	writer := gzip.NewWriter(h2)
	tee2 := io.TeeReader(reader, writer)

	return &RecompressibilityReader{tee2: tee2, h1: h1, reader: reader, writer: writer, h2: h2}, nil
}

// Read implements io.Reader
func (r RecompressibilityReader) Read(p []byte) (n int, err error) {
	return r.tee2.Read(p)
}

// Read implements io.Closer
func (r RecompressibilityReader) Close() error {
	err := r.reader.Close()
	if err != nil {
		return err
	}
	return r.writer.Close()
}

// TransparentlyRecompressible returns true if bytes read so far, once recompressed with compress/gzip's Writer,
// reconstruct the original archive exactly
func (r RecompressibilityReader) TransparentlyRecompressible() bool {
	return bytes.Equal(r.h1.Sum(nil), r.h2.Sum(nil))
}
