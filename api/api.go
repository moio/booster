package api

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"github.com/moio/booster/util"
	"github.com/rs/zerolog/log"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moio/booster/gzip"
	"github.com/moio/booster/wharf"
	"github.com/pkg/errors"
)

// Serve serves the HTTP API
func Serve(basedir string, port int, primary string) error {
	http.HandleFunc("/prepare_diff", func(writer http.ResponseWriter, request *http.Request) {
		if err := PrepareDiff(basedir, writer, request); err != nil {
			abort(err, writer)
		}
	})

	http.HandleFunc("/diff", func(writer http.ResponseWriter, request *http.Request) {
		if err := Diff(basedir, writer, request); err != nil {
			abort(err, writer)
		}
	})

	http.HandleFunc("/sync", func(writer http.ResponseWriter, request *http.Request) {
		if err := Sync(basedir, primary, writer, request); err != nil {
			abort(err, writer)
		}
	})

	http.HandleFunc("/cleanup", func(writer http.ResponseWriter, request *http.Request) {
		if err := Cleanup(basedir, writer, request); err != nil {
			abort(err, writer)
		}
	})

	log.Info().Msg("API started")

	return http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}

// PrepareDiffResp represents the json response of PrepareDiff
type PrepareDiffResp struct {
	Hash string
}

// PrepareDiff computes the patch between (decompressed) files in basedir and files passed in
// the request body.
// The result is cached in a temporary directory by hash, returned in the response body
func PrepareDiff(basedir string, w http.ResponseWriter, r *http.Request) error {
	// determine old files, passed as parameter
	oldFiles := util.NewFileSet()
	for _, f := range strings.Split(r.FormValue("old"), "\n") {
		oldFiles.Add(path.Join(basedir, f))
	}

	// determine new files, which is all files we have in decompressed form only
	newFiles, err := gzip.DecompressWalking(basedir)
	if err != nil {
		return errors.Wrap(err, "PrepareDiff: error while decompressing files")
	}

	// compute a unique hash for this diff
	h, err := hash(oldFiles, newFiles)
	if err != nil {
		return errors.Wrap(err, "PrepareDiff: error while computing hash")
	}

	// actually compute the diff, if new
	if err := os.MkdirAll(path.Join(basedir, "booster"), 0700); err != nil {
		return errors.Wrap(err, "PrepareDiff: error while creating 'booster' temporary directory")
	}

	log.Info().Str("hash", h[:10]).Msg("Creating patch...")

	patchPath := path.Join(basedir, "booster", h)
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		f, err := os.Create(patchPath)
		if err != nil {
			return errors.Wrap(err, "PrepareDiff: error while opening patch file")
		}
		oldFilter := wharf.NewFileSetFilter(oldFiles)
		newFilter := wharf.NewFileSetFilter(newFiles)
		err = wharf.CreatePatch(basedir, oldFilter.Filter, basedir, newFilter.Filter, util.PreventClosing(f))
		if err != nil {
			return errors.Wrap(err, "PrepareDiff: error while creating patch")
		}
		if err := f.Close(); err != nil {
			return errors.Wrap(err, "PrepareDiff: error while closing patch file")
		}
	}

	// return the unique hash in the response
	response, err := json.Marshal(PrepareDiffResp{Hash: h})
	if err != nil {
		return errors.Wrap(err, "PrepareDiff: error while marshalling response")
	}

	if _, err := w.Write(response); err != nil {
		return errors.Wrap(err, "PrepareDiff: error while writing response")
	}

	return nil
}

// Diff serves a patch previously computed via PrepareDiff. It expects a hash value as parameter
func Diff(basedir string, w http.ResponseWriter, r *http.Request) error {
	h := r.FormValue("hash")

	log.Info().Str("hash", h[:10]).Msg("Serving patch")

	// sanitize input
	if _, err := regexp.MatchString("[0-9a-f]", h); err != nil {
		return errors.Wrap(errors.Errorf("invalid hash %v", h), "Diff: hash validation error")
	}

	http.ServeFile(w, r, path.Join(basedir, "booster", h))
	return nil
}

// Sync requests the patch from the set of files in path to the set of files on the primary
// and applies it locally
func Sync(path string, primary string, w http.ResponseWriter, r *http.Request) error {
	// determine new files, which is all files we have in decompressed form only
	decompressed, err := gzip.DecompressWalking(path)
	if err != nil {
		return errors.Wrap(err, "Sync: error while decompressing files")
	}
	relative, err := decompressed.Relative(path)
	if err != nil {
		return errors.Wrap(err, "Sync: error while computing request to primary")
	}

	log.Info().Str("primary", primary).Msg("Requesting to prepare patch...")

	resp, err := http.PostForm(
		primary+"/prepare_diff",
		url.Values{"old": {strings.Join(relative.Sorted(), "\n")}})
	if err != nil {
		return errors.Wrap(err, "Sync: error requesting diff preparation to primary")
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "Sync: error getting diff preparation hash to primary")
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("Sync: obtained error from primary: %v", string(bodyBytes))
	}
	var prepareResp PrepareDiffResp
	if err := json.Unmarshal(bodyBytes, &prepareResp); err != nil {
		return errors.Wrap(err, "Sync: error unmarshaling hash from primary")
	}

	h := prepareResp.Hash
	log.Info().Str("hash", h[:10]).Msg("Downloading and applying patch...")

	tempDir := filepath.Join(os.TempDir(), "booster", "staging")

	size, err := wharf.Apply(primary+"/diff?hash="+h, path, tempDir)
	if err != nil {
		return errors.Wrap(err, "Sync: error while applying patch")
	}

	if err := gzip.RecompressAllIn(path); err != nil {
		return errors.Wrap(err, "Sync: error while recompressing files")
	}

	log.Info().Int64("transferred_MiB", size/1024/1024).Msg("Sync done")

	return nil
}

// Cleanup removes any booster-specific file
func Cleanup(basedir string, writer http.ResponseWriter, request *http.Request) error {
	if err := os.RemoveAll(path.Join(basedir, "booster")); err != nil {
		return err
	}
	return gzip.Clean(basedir)
}

// hash computes a hash from sets of paths
func hash(oldFiles *util.FileSet, newFiles *util.FileSet) (string, error) {
	h := sha512.New()
	for _, f := range oldFiles.Sorted() {
		if _, err := io.WriteString(h, f); err != nil {
			return "", err
		}
	}
	if _, err := io.WriteString(h, "//////"); err != nil {
		return "", err
	}
	for _, f := range newFiles.Sorted() {
		if _, err := io.WriteString(h, f); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// abort writes an error (500) response
func abort(err error, w http.ResponseWriter) {
	log.Error().Err(err).Stack().Send()
	w.WriteHeader(500)
	_, err = fmt.Fprintf(w, "Unexpected error: %v\n", err)
	if err != nil {
		log.Error().Err(err).Msg("additional error while writing the response")
	}
}
