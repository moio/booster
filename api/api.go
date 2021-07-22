package api

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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
	oldFiles := map[string]bool{}
	old := r.FormValue("old")
	for _, f := range strings.Split(old, "\n") {
		oldFiles[f] = true
	}

	// determine new files, which is all files we have in decompressed form only
	if err := gzip.DecompressAllIn(basedir); err != nil {
		return errors.Wrap(err, "PrepareDiff: error while decompressing files")
	}
	newFiles, err := gzip.ListDecompressedOnly(basedir)
	if err != nil {
		return errors.Wrap(err, "PrepareDiff: error while listing decompressed files")
	}

	// compute a unique hash for this diff
	h, err := hash(oldFiles, newFiles)
	if err != nil {
		return errors.Wrap(err, "PrepareDiff: error while computing hash")
	}

	// actually compute the diff, if new
	if err := os.MkdirAll(path.Join(os.TempDir(), "booster"), 0700); err != nil {
		return errors.Wrap(err, "PrepareDiff: error while creating 'booster' temporary directory")
	}
	patchPath := path.Join(os.TempDir(), "booster", h)
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		f, err := os.Create(patchPath)
		if err != nil {
			return errors.Wrap(err, "PrepareDiff: error while opening patch file")
		}
		oldFilter := wharf.NewAcceptListFilter(basedir, oldFiles)
		newFilter := wharf.NewAcceptListFilter(basedir, newFiles)
		err = wharf.CreatePatch(basedir, oldFilter.Filter, basedir, newFilter.Filter, wharf.PreventClosing(f))
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

	// sanitize input
	if _, err := regexp.MatchString("[0-9a-f]", h); err != nil {
		return errors.Wrap(errors.Errorf("invalid hash %v", h), "Diff: hash validation error")
	}

	http.ServeFile(w, r, path.Join(os.TempDir(), "booster", h))
	return nil
}

// SyncResp represents the json response of Sync
type SyncResp struct {
	TransferredMB int64
}

// Sync requests the patch from the set of files in path to the set of files on the primary
// and applies it locally
func Sync(path string, primary string, w http.ResponseWriter, r *http.Request) error {
	// determine new files, which is all files we have in decompressed form only
	if err := gzip.DecompressAllIn(path); err != nil {
		return errors.Wrap(err, "Sync: error while decompressing files")
	}
	decompressed, err := gzip.ListDecompressedOnly(path)
	if err != nil {
		return errors.Wrap(err, "Sync: error while listing decompressed files")
	}
	old := sorted(decompressed)

	resp, err := http.PostForm(
		filepath.Join(primary, "prepare_diff"),
		url.Values{"old": {strings.Join(old, "\n")}})
	if err != nil {
		return errors.Wrap(err, "Sync: error requesting diff preparation to primary")
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "Sync: error getting diff preparation hash to primary")
	}
	var response PrepareDiffResp
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return errors.Wrap(err, "Sync: error unmarshaling hash from primary")
	}

	size, err := wharf.Apply(primary+"/diff?hash="+response.Hash, path)
	if err != nil {
		return errors.Wrap(err, "Sync: error while applying patch")
	}

	if err := gzip.RecompressAllIn(path); err != nil {
		return errors.Wrap(err, "Sync: error while recompressing files")
	}

	json, err := json.MarshalIndent(SyncResp{TransferredMB: size / 1024 / 1024}, "", "  ")
	if err != nil {
		return errors.Wrap(err, "Sync: error marshalling response")
	}

	if _, err := w.Write(json); err != nil {
		return errors.Wrap(err, "Sync: error while writing response")
	}

	return nil
}

// sorted turns a path set into a path list
func sorted(pathSet map[string]bool) []string {
	result := []string{}
	for k := range pathSet {
		result = append(result, k)
	}
	sort.Strings(result)

	return result
}

// hash computes a hash from sets of paths
func hash(oldMap map[string]bool, newMap map[string]bool) (string, error) {
	h := sha512.New()
	for _, f := range sorted(oldMap) {
		if _, err := io.WriteString(h, f); err != nil {
			return "", err
		}
	}
	if _, err := io.WriteString(h, "//////"); err != nil {
		return "", err
	}
	for _, f := range sorted(newMap) {
		if _, err := io.WriteString(h, f); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// abort writes an error (500) response
func abort(err error, w http.ResponseWriter) {
	w.WriteHeader(500)
	fmt.Fprintf(w, "Unexpected error: %v\n", err)
	log.Print(err)
}
