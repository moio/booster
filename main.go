package main

import (
	"fmt"
	"github.com/moio/booster/api"
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
	app.Action = serve
	app.ArgsUsage = "dir"

	app.Before = func(c *cli.Context) error {
		if len(c.Args()) < 1 {
			return errors.New("Usage: booster dir")
		}
		return nil
	}
	app.Flags = []cli.Flag{
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

	return api.Serve(path, ctx.Int("port"), ctx.String("primary"))
}
