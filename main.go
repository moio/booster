package main

import (
	"fmt"
	"github.com/moio/booster/api"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// set by goreleaser via -ldflags at build time
// see https://golang.org/cmd/link/, https://goreleaser.com/customization/build/
// Empty string means snapshot build
var version string

func main() {
	app := cli.NewApp()
	app.Name = "booster"
	app.Usage = "Synchronizes container image registries efficiently"
	if version != "" {
		app.Version = version
	} else {
		app.Version = "snapshot"
	}
	app.EnableBashCompletion = true

	app.Commands = []cli.Command{
		{
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
					Usage: "TCP port for the API (default 5000)",
					Value: 5000,
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

func serve(ctx *cli.Context) error {
	path := ctx.Args().First()
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.Errorf("%v is not a directory", path)
	}

	http.HandleFunc("/prepare_diff", func(writer http.ResponseWriter, request *http.Request) {
		api.PrepareDiff(path, writer, request)
	})

	http.HandleFunc("/patch", func(writer http.ResponseWriter, request *http.Request) {
		api.Patch(path, writer, request)
	})

	http.HandleFunc("/sync", func(writer http.ResponseWriter, request *http.Request) {
		primary := ctx.String("primary")
		api.Sync(path, primary, writer, request)
	})
	return http.ListenAndServe(fmt.Sprintf(":%v", ctx.Int("port")), nil)
}
