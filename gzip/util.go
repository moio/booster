package gzip

import (
	"compress/gzip"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/alitto/pond"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Suffix is the name appended to files decompressed by this module
const Suffix = "_UNGZIPPED_BY_BOOSTER"

// DecompressAllIn uncompresses "recompressible" gzip files found in basePath and subdirectories
func DecompressAllIn(basePath string) error {
	log.Debug().Msg("Decompressing layer files...")
	pool := pond.New(runtime.NumCPU(), 1000)
	err := filepath.WalkDir(basePath, func(p string, d fs.DirEntry, err error) error {
		// skip irregular files
		if !d.Type().IsRegular() {
			return nil
		}
		// skip already decompressed (by name)
		if strings.HasSuffix(p, Suffix) {
			return nil
		}
		// skip already decompressed (by decompressed file existence)
		uncompressedPath := p + Suffix
		if _, err := os.Stat(uncompressedPath); err == nil {
			return nil
		}

		pool.Submit(func() {
			if err := decompress(p, uncompressedPath); err != nil {
				log.Error().Err(err)
			}
		})
		return nil
	})
	if err != nil {
		return err
	}

	pool.StopAndWait()
	if pool.FailedTasks() != 0 {
		return errors.Errorf("Error while decompressing files in %v", basePath)
	}
	return nil
}

// decompress decompresses a gzip file, if recompressible, into destinationPath
func decompress(sourcePath string, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "could not open to attempt decompression: %v", sourcePath)
	}

	rreader, err := NewRecompressibilityReader(source)
	if err == gzip.ErrHeader || err == io.EOF {
		// not a gzip or even empty, situation normal
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "error while initing decompression: %v", sourcePath)
	}

	destination, err := os.Create(destinationPath)
	if err != nil {
		return errors.Wrapf(err, "could not create temporary file to attempt decompression: %v", destinationPath)
	}

	_, err = io.Copy(destination, rreader)
	if err != nil {
		return errors.Wrapf(err, "error while decompressing: %v", sourcePath)
	}

	err = rreader.Close()
	if err != nil {
		return errors.Wrapf(err, "error while closing: %v", sourcePath)
	}
	err = destination.Close()
	if err != nil {
		return errors.Wrapf(err, "error while closing: %v", destinationPath)
	}
	err = source.Close()
	if err != nil {
		return errors.Wrapf(err, "error while closing: %v", sourcePath)
	}

	if !rreader.TransparentlyRecompressible() {
		// decompression worked but the result can't be compressed back
		// this archive can't be trusted, roll back
		os.Remove(destinationPath)
	}

	return nil
}

// RecompressAllIn recompresses any gzip files decompressed by DecompressAllIn
func RecompressAllIn(basePath string) error {
	log.Debug().Msg("Recompressing layer files...")
	pool := pond.New(runtime.NumCPU(), 1000)
	err := filepath.WalkDir(basePath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip any file other than those created by DecompressAllIn
		if !strings.HasSuffix(p, Suffix) {
			return nil
		}
		// skip already compressed
		compressedPath := strings.TrimSuffix(p, Suffix)
		if _, err := os.Stat(compressedPath); err == nil {
			return nil
		}

		pool.Submit(func() {
			if err := compress(p, compressedPath); err != nil {
				log.Error().Err(err)
			}
		})
		return nil
	})
	if err != nil {
		return err
	}

	pool.StopAndWait()
	if pool.FailedTasks() != 0 {
		return errors.Errorf("Error while decompressing files in %v", basePath)
	}
	return nil
}

// compress gzip-compresses a file
func compress(sourcePath string, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "could not open to compress: %v", sourcePath)
	}

	destination, err := os.Create(destinationPath)
	if err != nil {
		return errors.Wrapf(err, "could not open to compress: %v", destinationPath)
	}

	gzDestination := gzip.NewWriter(destination)

	_, err = io.Copy(gzDestination, source)
	if err != nil {
		return errors.Wrapf(err, "error while compressing: %v", sourcePath)
	}

	err = gzDestination.Close()
	if err != nil {
		return errors.Wrapf(err, "error while closing: %v", destinationPath)
	}
	err = destination.Close()
	if err != nil {
		return errors.Wrapf(err, "error while closing: %v", destinationPath)
	}
	err = source.Close()
	if err != nil {
		return errors.Wrapf(err, "error while closing: %v", sourcePath)
	}

	return nil
}

// ListDecompressedOnly returns a set of all files in a directory
func ListDecompressedOnly(path string) (map[string]bool, error) {
	current := map[string]bool{}
	toRemove := []string{}
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative, err := filepath.Rel(path, p)
		if err != nil {
			return errors.Wrapf(err, "Cannot compute relative path of %s", filepath.Join(path, p))
		}

		// skip the booster-specific dir altogether
		if strings.Split(relative, string(os.PathSeparator))[0] == "booster" {
			return nil
		}

		current[relative] = true
		if strings.HasSuffix(relative, Suffix) {
			toRemove = append(toRemove, strings.TrimSuffix(relative, Suffix))
		}
		return nil
	})
	if err != nil {
		return make(map[string]bool), err
	}

	// remove files for which we have an uncompressed copy
	for _, k := range toRemove {
		delete(current, k)
	}

	return current, nil
}

// Clean deletes decompressed files
func Clean(path string) error {
	log.Info().Str("path", path).Msg("Cleaning")
	var toRemove []string
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(p, Suffix) {
			toRemove = append(toRemove, p)
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "error while walking files to clean")
	}

	// remove files for which we have an uncompressed copy
	for _, k := range toRemove {
		if err := os.Remove(k); err != nil {
			return errors.Wrapf(err, "error while deleting file %s", k)
		}
	}

	return nil
}
