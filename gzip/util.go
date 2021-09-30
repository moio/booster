package gzip

import (
	"compress/gzip"
	"github.com/alitto/pond"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Suffix is the name appended to files decompressed by this module
const Suffix = "_UNGZIPPED_BY_BOOSTER"

// DecompressWalking decompresses "recompressible" gzip files found in basePath and subdirectories
func DecompressWalking(basePath string) (map[string]bool, error) {
	paths := map[string]bool{}
	err := filepath.WalkDir(basePath, func(p string, d fs.DirEntry, err error) error {
		// skip the booster-specific dir altogether
		if d.Type().IsDir() && d.Name() == "booster" {
			return fs.SkipDir
		}
		// skip irregular files
		if !d.Type().IsRegular() {
			return nil
		}
		// skip already decompressed files (by name)
		if strings.HasSuffix(p, Suffix) {
			return nil
		}

		relative, rerr := filepath.Rel(basePath, p)
		if rerr != nil {
			return errors.Wrapf(rerr, "Cannot compute relative path of %s", p)
		}
		paths[relative] = true

		return nil
	})
	if err != nil {
		return nil, err
	}

	return Decompress(paths, basePath), nil
}

// Decompress decompresses "recompressible" gzip files in the specified map
// uses up to runtime.NumCPU()*2 goroutines concurrently, one per file
// returns a map of decompressed or unchanged paths
func Decompress(paths map[string]bool, basePath string) map[string]bool {
	log.Debug().Msg("Decompressing layers...")

	processedPaths := make(chan string, runtime.NumCPU()*2)
	for path := range paths {
		originalPath := path // https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		uncompressedPath := path + Suffix
		go func() {
			if decompress(filepath.Join(basePath, originalPath), filepath.Join(basePath, uncompressedPath)) {
				// decompression was successful, return path to decompressed file
				processedPaths <- uncompressedPath
			} else {
				// decompression was NOT successful, return the original path
				processedPaths <- originalPath
			}
		}()
	}

	// make a map of all processed paths
	result := make(map[string]bool)
	for i := 0; i < len(paths); i++ {
		processedPath := <-processedPaths
		result[processedPath] = true

		// add also parent dirs
		for processedPath != "." {
			processedPath = filepath.Dir(processedPath)
			result[processedPath] = true
		}
	}

	return result
}

// decompress decompresses a gzip file, if recompressible, into destinationPath
// returns true in case decompression was successful, false if the decompression could not happen
// (or could happen but without recompressibility guarantees)
// any errors are logged and not returned
func decompress(sourcePath string, destinationPath string) bool {
	if _, err := os.Stat(destinationPath); err == nil {
		// file has been decompressed already
		return true
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		log.Error().Str("path", sourcePath).Err(err).Msg("could not open to attempt decompression")
		return false
	}

	rreader, err := NewRecompressibilityReader(source)
	if err == gzip.ErrHeader || err == io.EOF {
		// not a gzip or even empty, situation normal
		closeAndLog(source)
		return false
	}
	if err != nil {
		log.Error().Str("path", sourcePath).Err(err).Msg("error while initing decompression")
		closeAndLog(source)
		return false
	}

	destination, err := os.Create(destinationPath)
	if err != nil {
		log.Error().Str("path", destinationPath).Err(err).Msg("could not create temporary file to attempt decompression")
		closeAndLog(source)
		return false
	}

	_, err = io.Copy(destination, rreader)
	if err != nil {
		log.Error().Str("path", sourcePath).Err(err).Msg("error while decompressing")
		closeAndLog(destination)
		removeAndLog(destinationPath)
		closeAndLog(source)
		return false
	}

	err = rreader.Close()
	if err != nil {
		log.Error().Str("path", sourcePath).Err(err).Msg("error while closing recompressibility reader")
		closeAndLog(destination)
		removeAndLog(destinationPath)
		closeAndLog(source)
		return false
	}

	closeAndLog(destination)
	closeAndLog(source)

	if !rreader.TransparentlyRecompressible() {
		// decompression worked but the result can't be compressed back
		// this archive can't be trusted, roll back
		removeAndLog(destinationPath)
		return false
	}

	return true
}

// closeAndLog closes a file logging any errors
func closeAndLog(f *os.File) {
	err := f.Close()
	if err != nil {
		log.Error().Str("path", f.Name()).Err(err).Msg("error while closing")
	}
}

// removeAndLog removes a file logging any errors
func removeAndLog(path string) {
	if err := os.Remove(path); err != nil {
		log.Error().Str("path", path).Err(err).Msg("error while removing")
	}
}

// RecompressAllIn recompresses any gzip files decompressed by Decompress
func RecompressAllIn(basePath string) error {
	log.Debug().Msg("Recompressing layer files...")
	pool := pond.New(runtime.NumCPU(), 1000)
	err := filepath.WalkDir(basePath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip any file other than those created by Decompress
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
