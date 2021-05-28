package main

import (
	"fmt"
	"github.com/moio/regsync/streams"
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
			Name:      "compress",
			Usage:     "compresses a file with go's gzip to standard output",
			ArgsUsage: "[filename (default stdin)] [output file (default filename.gz or stdout)]",
			Action:    compress,
		},
		cli.Command{
			Name:      "check",
			Usage:     "decompresses and recompresses a file with go's gzip. Exits with 0 if recompression was transparent",
			ArgsUsage: "[filename (default stdin)]",
			Action:    check,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func compress(ctx *cli.Context) error {
	var input io.Reader
	var output io.Writer
	if len(ctx.Args()) == 0 {
		input = os.Stdin
		output = os.Stdout
	} else {
		var err error
		input, err = os.Open(ctx.Args().First())
		if err != nil {
			return err
		}

		outputName := ctx.Args().First() + ".gz"
		if len(ctx.Args()) > 1 {
			outputName = ctx.Args().Get(1)
		}

		output, err = os.Create(outputName)
		if err != nil {
			return err
		}
	}

	return streams.Compress(input, output)
}

func check(ctx *cli.Context) error {
	var input io.Reader
	if len(ctx.Args()) == 0 {
		input = os.Stdin
	} else {
		var err error
		input, err = os.Open(ctx.Args().First())
		if err != nil {
			return err
		}
	}

	result, err := streams.IsRecompressible(input)
	if err != nil {
		return err
	}

	if !result {
		return errors.New("Archive is NOT recompressible!")
	}

	fmt.Println("Archive is recompressible!")
	return nil
}
