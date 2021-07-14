package api

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"github.com/moio/booster/gzip"
	"github.com/moio/booster/wharf"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Serve serves the HTTP API
func Serve(basedir string, port int, primary string) error {
	http.HandleFunc("/prepare_diff", func(writer http.ResponseWriter, request *http.Request) {
		PrepareDiff(basedir, writer, request)
	})

	http.HandleFunc("/diff", func(writer http.ResponseWriter, request *http.Request) {
		Diff(basedir, writer, request)
	})

	http.HandleFunc("/sync", func(writer http.ResponseWriter, request *http.Request) {
		Sync(basedir, primary, writer, request)
	})

	return http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
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
	err := gzip.DecompressAllIn(basedir)
	if err != nil {
		return bark(err, "PrepareDiff: error while decompressing files", w)
	}
	newFiles := gzip.ListDecompressedOnly(basedir)

	// compute a unique hash for this diff
	h := hash(oldFiles, newFiles)

	// actually compute the diff, if new
	os.MkdirAll(path.Join(os.TempDir(), "booster"), 0700)
	patchPath := path.Join(os.TempDir(), "booster", h)
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		f, err := os.Create(patchPath)
		if err != nil {
			return bark(err, "PrepareDiff: error while opening patch file", w)
		}
		oldFilter := wharf.NewAcceptListFilter(basedir, oldFiles)
		newFilter := wharf.NewAcceptListFilter(basedir, newFiles)
		err = wharf.CreatePatch(basedir, oldFilter.Filter, basedir, newFilter.Filter, wharf.PreventClosing(f))
		if err != nil {
			return bark(err, "PrepareDiff: error while creating patch", w)
		}
		err = f.Close()
		if err != nil {
			return bark(err, "PrepareDiff: error while closing patch file", w)
		}
	}

	// return the unique hash in the response
	response, _ := json.Marshal(map[string]string{"hash": h})
	_, err = w.Write(response)
	if err != nil {
		return bark(err, "PrepareDiff: error while writing response", w)
	}

	return err
}

// Diff serves a patch previously computed via PrepareDiff. It expects a hash value as parameter
func Diff(basedir string, w http.ResponseWriter, r *http.Request) error {
	h := r.FormValue("hash")

	// sanitize input
	_, err := regexp.MatchString("[0-9a-f]", h)
	if err != nil {
		return bark(errors.Errorf("invalid hash %v", h), "Diff: hash validation error", w)
	}

	http.ServeFile(w, r, path.Join(os.TempDir(), "booster", h))
	return nil
}

// Sync requests the patch from the sert of files in path to the set of files on the primary
// and applies it locally
func Sync(path string, primary string, w http.ResponseWriter, r *http.Request) error {
	// determine new files, which is all files we have in decompressed form only
	err := gzip.DecompressAllIn(path)
	if err != nil {
		return bark(err, "Sync: error while decompressing files", w)
	}
	old := sorted(gzip.ListDecompressedOnly(path))

	resp, err := http.PostForm(primary+"/prepare_diff", url.Values{"old": {strings.Join(old, "\n")}})
	if err != nil {
		return bark(err, "Sync: error requesting diff preparation to primary", w)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return bark(err, "Sync: error getting diff preparation hash to primary", w)
	}
	var response map[string]string
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return bark(err, "Sync: error unmarshaling hash from primary", w)
	}
	h := response["hash"]

	size, err := wharf.Apply(primary+"/diff?hash="+h, path)
	if err != nil {
		return bark(err, "Sync: error while applying patch", w)
	}

	err = gzip.RecompressAllIn(path)
	if err != nil {
		return bark(err, "Sync: error while recompressing files", w)
	}

	json, _ := json.MarshalIndent(map[string]int64{"transferred_mb": size / 1024 / 1024}, "", "  ")
	_, err = w.Write(json)
	if err != nil {
		return bark(err, "Sync: error while writing response", w)
	}

	return nil
}

// sorted turns a path set into a path list
func sorted(pathSet map[string]bool) []string {
	result := make([]string, 0)
	for k, _ := range pathSet {
		result = append(result, k)
	}
	sort.Strings(result)

	return result
}

// hash computes a hash from sets of paths
func hash(oldMap map[string]bool, newMap map[string]bool) string {
	h := sha512.New()
	for _, f := range sorted(oldMap) {
		io.WriteString(h, f)
	}
	io.WriteString(h, "//////")
	for _, f := range sorted(newMap) {
		io.WriteString(h, f)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// bark writes an error (500) response
func bark(e error, message string, w http.ResponseWriter) error {
	err := errors.Wrap(e, message)
	w.WriteHeader(500)
	fmt.Fprintf(w, "Unexpected error: %v\n", err)
	log.Print(err)
	return err
}
