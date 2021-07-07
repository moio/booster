package main

import (
	"fmt"
	"github.com/itchio/lake/tlc"
	"github.com/moio/booster/api"
	"github.com/moio/booster/streams"
	"github.com/moio/booster/wharf"
	"io"
	"net/http"
	"strings"

	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "booster"
	app.Usage = "Synchronizes container image registries efficiently"
	app.Version = "0.1"
	app.EnableBashCompletion = true

	app.Commands = []cli.Command{
		cli.Command{
			Name:      "compress",
			Usage:     "compresses a file with go's gzip to standard output",
			ArgsUsage: "[filename (default stdin)] [output file (default filename.gz or stdout)]",
			Action:    compress,
		},
		cli.Command{
			Name:      "decompress",
			Usage:     "decompresses a file with go's gzip to standard output",
			ArgsUsage: "[filename (default stdin)] [output file (default filename witout .gz or stdout)]",
			Action:    decompress,
		},
		cli.Command{
			Name:      "isgzip",
			Usage:     "Exists with 0 if input file is gzipped",
			ArgsUsage: "[filename (default stdin)]",
			Action:    isGzip,
		},
		cli.Command{
			Name:      "recompressible",
			Usage:     "decompresses and recompresses a file with go's gzip. Exits with 0 if recompression was transparent",
			ArgsUsage: "[filename (default stdin)]",
			Action:    recompressible,
		},
		cli.Command{
			Name:      "diff",
			Usage:     "creates a delta via the wharf library between two directories",
			ArgsUsage: "old_dir new_dir [diff_filename (default stdout)]",
			Action:    diff,
			Before: func(c *cli.Context) error {
				if len(c.Args()) < 2 {
					return errors.New("Usage: booster diff old_dir new_dir [diff_filename (default stdout)]")
				}
				return nil
			},
		},
		cli.Command{
			Name:      "apply",
			Usage:     "applies a delta created with the wharf library to a directory",
			ArgsUsage: "diff_filename dir",
			Action:    apply,
			Before: func(c *cli.Context) error {
				if len(c.Args()) != 2 {
					return errors.New("Usage: booster apply diff_filename dir")
				}
				return nil
			},
		},
		cli.Command{
			Name:      "serve",
			Usage:     "serves the booster HTTP API",
			ArgsUsage: "dir",
			Action:    serve,
			Before: func(c *cli.Context) error {
				if len(c.Args()) < 1 {
					return errors.New("Usage: booster serve dir")
				}
				return nil
			},
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "port",
					Usage: "TCP port for the API (default 8000)",
					Value: 8000,
				},
				cli.StringFlag{
					Name:  "primary",
					Usage: "http address of the primary, if any",
					Value: "",
				},
			},
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

func decompress(ctx *cli.Context) error {
	var input io.Reader
	var output io.Writer
	if len(ctx.Args()) == 0 {
		input = os.Stdin
		output = os.Stdout
	} else {
		inputName := ctx.Args().First()
		var err error
		input, err = os.Open(inputName)
		if err != nil {
			return err
		}

		var outputName string
		if len(ctx.Args()) > 1 {
			outputName = ctx.Args().Get(1)
		} else if strings.HasSuffix(inputName, ".gz") {
			outputName = strings.TrimSuffix(inputName, ".gz")
		} else {
			outputName = inputName + "-decompressed"
		}

		output, err = os.Create(outputName)
		if err != nil {
			return err
		}
	}

	return streams.Decompress(input, output)
}

func isGzip(ctx *cli.Context) error {
	var path string
	var input io.Reader
	if len(ctx.Args()) == 0 {
		path = "-"
		input = os.Stdin
	} else {
		var err error
		path = ctx.Args().First()
		input, err = os.Open(path)
		if err != nil {
			return err
		}
	}

	result, err := streams.IsGzip(input)
	if err != nil {
		return err
	}

	if !result {
		return errors.New("Archive is not gzip")
	}

	fmt.Println(path)
	return nil
}

func recompressible(ctx *cli.Context) error {
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

	result, err := streams.Recompressible(input)
	if err != nil {
		return err
	}

	if !result {
		return errors.New("Archive is NOT recompressible!")
	}

	fmt.Println("Archive is recompressible!")
	return nil
}

func diff(ctx *cli.Context) error {
	oldPath := ctx.Args().First()
	oldInfo, err := os.Stat(oldPath)
	if err != nil {
		return err
	}
	if !oldInfo.IsDir() {
		return errors.Errorf("%v is not a directory", oldPath)
	}

	newPath := ctx.Args().Get(1)
	newInfo, err := os.Stat(newPath)
	if err != nil {
		return err
	}
	if !newInfo.IsDir() {
		return errors.Errorf("%v is not a directory", newPath)
	}

	var output io.Writer
	if len(ctx.Args()) == 2 {
		output = os.Stdout
	} else {
		output, err = os.Create(ctx.Args().Get(2))
		if err != nil {
			return err
		}
	}

	return wharf.CreatePatch(oldPath, tlc.KeepAllFilter, newPath, output)
}

func apply(ctx *cli.Context) error {
	patchPath := ctx.Args().First()

	dirPath := ctx.Args().Get(1)
	newInfo, err := os.Stat(dirPath)
	if err != nil {
		return err
	}
	if !newInfo.IsDir() {
		return errors.Errorf("%v is not a directory", dirPath)
	}

	return wharf.Apply(patchPath, dirPath)
}

func serve(ctx *cli.Context) error {
	path := ctx.Args().First()
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.Errorf("%v is not a directory", path)
	}

	http.HandleFunc("/diff", func(writer http.ResponseWriter, request *http.Request) {
		api.Diff(path, writer, request)
	})

	http.HandleFunc("/sync", func(writer http.ResponseWriter, request *http.Request) {
		primary := ctx.String("primary")
		api.Sync(path, primary, writer, request)
	})
	return http.ListenAndServe(fmt.Sprintf(":%v", ctx.Int("port")), nil)
}
