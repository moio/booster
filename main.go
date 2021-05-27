package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"fmt"

	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "regsync"
	app.Usage = "Utility to synchronize container image registries"
	app.Version = "0.1"

	app.Commands = []cli.Command{
		cli.Command{
			Name:   "compress",
			Usage:  "compresses standard input with go's gzip to standard output",
			Action: compress,
		},
		cli.Command{
			Name:   "check",
			Usage:  "decompresses and recompresses standard input with go's gzip. Exits with 0 if recompression was transparent",
			Action: check,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func compress(ctx *cli.Context) error {
	buf := make([]byte, 1024*1024)
	w := gzip.NewWriter(os.Stdout)

	_, err := io.CopyBuffer(w, os.Stdin, buf)
	if err != nil {
		return errors.Wrap(err, "error while compressing stream")
	}

	if err := w.Close(); err != nil {
		return errors.Wrap(err, "error while closing compressed stream")
	}
	return nil
}

func check(ctx *cli.Context) error {
	// stdin -> tee -> originalHash
	//           |---> gzip reader -> gzip writer -> pipe -> recompressedHash
	buf := make([]byte, 1024*1024)
	originalHash := md5.New()
	tr := io.TeeReader(os.Stdin, originalHash)
	gzipReader, err := gzip.NewReader(tr)
	if err != nil {
		return errors.Wrap(err, "error while closing compressed stream")
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
		return errors.Wrap(err, "error while hashing stream")
	}

	if err := pipeReader.Close(); err != nil {
		return errors.Wrap(err, "error while close hashing stream")
	}

	err = <-result
	if err != nil {
		return err
	}

	if !bytes.Equal(originalHash.Sum(nil), recompressedHash.Sum(nil)) {
		return errors.New("Archive is NOT reconstructable!")
	}

	fmt.Println("Archive is reconstructable!")
	return nil
}
