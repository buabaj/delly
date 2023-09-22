package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/urfave/cli/v2"
)

type dirMap map[string]dirMeta

type dirMeta struct {
	size         int64
	bytesDeleted int64
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:     "ext",
				Aliases:  []string{"e"},
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"dry"},
			},
		},
		Before: func(ctx *cli.Context) error {
			args := ctx.Args()
			if args.Len() != 1 {
				return errors.New("error invalid args: exactly one argument must be provided")
			}
			return nil
		},
		Action: func(ctx *cli.Context) error {
			exts := ctx.StringSlice("ext")
			dry := ctx.Bool("dry-run")
			rootDir := ctx.Args().Get(0)

			if dry {
				if err := dryrun(rootDir, exts); err != nil {
					return err
				}

				return nil
			}

			if err := deleteFiles(rootDir, exts); err != nil {
				return err
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func (d dirMap) report() error {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "DIRECTORY\tOLDSIZE\tNEWSIZE\tBYTES SAVED\n")
	fmt.Fprint(w, "---------\t-------\t-------\t-----------\n")
	for k, v := range d {
		if v.bytesDeleted != 0 {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\n",
				k,
				v.size,
				v.size-v.bytesDeleted,
				v.bytesDeleted,
			)
		}
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func dryrun(rootDir string, exts []string) error {
	fmap, sz, err := collectFileSizes(rootDir, exts)
	if err != nil {
		return err
	}

	if err := fmap.report(sz); err != nil {
		return err
	}

	return nil
}

func deleteFiles(rootDir string, exts []string) error {
	dmap, err := collectDirSizes(rootDir)
	if err != nil {
		return err
	}

	if err := deleteFilesByExtension(rootDir, exts, dmap); err != nil {
		return err
	}

	if err := dmap.report(); err != nil {
		return err
	}

	return nil
}

func (f fileMap) report(total int64) error {
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "FILE\tSIZE\n")
	fmt.Fprint(w, "----\t----\n")
	for k, v := range f {
		fmt.Fprintf(w, "%s\t%d\n", k, v)
	}

	fmt.Fprint(w, "----\t----\n")
	fmt.Fprintf(w, "TOTAL\t%d\n", total)

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}

	return nil
}

func deleteFilesByExtension(dir string, ext []string, dmap dirMap) error {
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
		}

		if !info.IsDir() {
			if matchExt(info.Name(), ext) {
				dir := filepath.Dir(path)
				sz, ok := dmap[dir]
				if ok {
					log.Printf("deleting %s\n", path)
					err := os.Remove(path)
					if err != nil {
						return err
					}
					sz.bytesDeleted += info.Size()
					dmap[dir] = sz
				}
			}
		}
		return nil
	})

	return nil
}

func collectDirSizes(rootDir string) (dirMap, error) {
	dirSz := make(dirMap)

	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			var d dirMeta
			dirSz[path] = d
			return nil
		}

		dir := filepath.Dir(path)
		sz, ok := dirSz[dir]
		if ok {
			sz.size += info.Size()
			dirSz[dir] = sz
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return dirSz, nil
}

type fileMap map[string]int64

func collectFileSizes(rootDir string, exts []string) (fileMap, int64, error) {
	fMeta := make(fileMap)
	var total int64

	err := filepath.Walk(rootDir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() && matchExt(info.Name(), exts) {
			size := info.Size()
			fMeta[path] = size
			total += size
		}

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	return fMeta, total, nil
}

func matchExt(file string, ext []string) bool {
	for _, e := range ext {
		if filepath.Ext(file) == e {
			return true
		}
	}
	return false
}
