package main

import (
	"encoding/gob"
	"fmt"
	"github.com/moio/regsync/rsync"
	"github.com/moio/regsync/streams"
	"io"
	"strings"

	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "regsync"
	app.Usage = "Utility to synchronize container image registries"
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
			Usage:     "creates a delta via the rsync algorithm between two files",
			ArgsUsage: "filename1 filename2 [diff_filename (default stdout)]",
			Action:    diff,
			Before: func(c *cli.Context) error {
				if len(c.Args()) < 2 {
					return errors.New("Usage: regsync filename1 filename2 [diff_filename (default stdout)]")
				}
				return nil
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
	first, err := os.Open(ctx.Args().First())
	if err != nil {
		return err
	}

	signature, err := rsync.CreateSignature(first)
	if err != nil {
		return err
	}

	second, err := os.Open(ctx.Args().Get(1))
	if err != nil {
		return err
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

	enc := gob.NewEncoder(output)
	writer := func(op rsync.Operation) error {
		err := enc.Encode(op)
		if err != nil {
			return err
		}
		return nil
	}

	return rsync.CreateDelta(second, signature, writer)
}
