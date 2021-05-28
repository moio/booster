package streams

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"github.com/pkg/errors"
	"io"
)

// Compress takes all bytes from a reader and gzips them into a writer
func Compress(reader io.Reader, writer io.Writer) error {
	buf := make([]byte, 1024*1024)
	gzipWriter := gzip.NewWriter(writer)
	_, err := io.CopyBuffer(gzipWriter, reader, buf)
	if err != nil {
		return errors.Wrap(err, "error while compressing stream")
	}

	if err := gzipWriter.Close(); err != nil {
		return errors.Wrap(err, "error while closing compressed stream")
	}
	return nil
}

// Decompress takes all bytes from a reader and ungzips them into a writer
func Decompress(reader io.Reader, writer io.Writer) error {
	buf := make([]byte, 1024*1024)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return errors.Wrap(err, "error while opening decompression stream")
	}
	_, err = io.CopyBuffer(writer, gzipReader, buf)
	if err != nil {
		return errors.Wrap(err, "error while decompressing stream")
	}

	if err := gzipReader.Close(); err != nil {
		return errors.Wrap(err, "error while closing decompressing stream")
	}
	return nil
}

// Recompressible decompresses bytes from a reader and checks whether they can be decompressed and recompressed to
// get the same archive as a result
func Recompressible(reader io.Reader) (bool, error) {
	// stdin -> tee -> originalHash
	//           |---> gzip reader -> gzip writer -> pipe -> recompressedHash

	buf := make([]byte, 1024*1024)
	originalHash := md5.New()
	tr := io.TeeReader(reader, originalHash)
	gzipReader, err := gzip.NewReader(tr)
	if err != nil {
		return false, errors.Wrap(err, "error while closing compressed stream")
	}

	pipeReader, pipeWriter := io.Pipe()

	result := make(chan error, 1)
	go func(result chan error) {
		buf2 := make([]byte, 1024*1024)
		gzipWriter := gzip.NewWriter(pipeWriter)
		_, err := io.CopyBuffer(gzipWriter, gzipReader, buf2)
		if err != nil {
			result <- errors.Wrap(err, "error while recompressing stream")
		}

		if err := gzipReader.Close(); err != nil {
			result <- errors.Wrap(err, "error while closing recompressing stream")
		}
		if err := gzipWriter.Close(); err != nil {
			result <- errors.Wrap(err, "error while closing recompressing stream")
		}
		if err := pipeWriter.Close(); err != nil {
			result <- errors.Wrap(err, "error while closing recompressing stream")
		}
		result <- nil
	}(result)

	recompressedHash := md5.New()
	_, err = io.CopyBuffer(recompressedHash, pipeReader, buf)
	if err != nil {
		return false, errors.Wrap(err, "error while hashing stream")
	}

	if err := pipeReader.Close(); err != nil {
		return false, errors.Wrap(err, "error while close hashing stream")
	}

	err = <-result
	if err != nil {
		return false, err
	}

	return bytes.Equal(originalHash.Sum(nil), recompressedHash.Sum(nil)), nil
}
