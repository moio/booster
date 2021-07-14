package api

import (
	"crypto/sha512"
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

// PrepareDiff computes the patch between (decompressed) files in basedir and files passed in
// the request body.
// The result is cached in a temporary directory by hash, returned in the response body
func PrepareDiff(basedir string, w http.ResponseWriter, r *http.Request) {
	// determine old files, passed as parameter
	oldFiles := map[string]bool{}
	old := r.FormValue("old")
	for _, f := range strings.Split(old, "\n") {
		oldFiles[f] = true
	}

	// determine new files, which is all files we have in decompressed form only
	err := gzip.DecompressAllIn(basedir)
	if err != nil {
		bark(err, w)
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
			bark(err, w)
			return
		}
		oldFilter := wharf.NewAcceptListFilter(basedir, oldFiles)
		newFilter := wharf.NewAcceptListFilter(basedir, newFiles)
		wharf.CreatePatch(basedir, oldFilter.Filter, basedir, newFilter.Filter, wharf.PreventClosing(f))
		err = f.Close()
		if err != nil {
			bark(err, w)
			return
		}
	}

	// return the unique hash in the response
	io.WriteString(w, h)
}

// Patch serves a patch previously computed via PrepareDiff. It expects a hash value as parameter
func Patch(basedir string, w http.ResponseWriter, r *http.Request) {
	h := r.FormValue("hash")

	// sanitize input
	_, err := regexp.MatchString("[0-9a-f]", h)
	if err != nil {
		bark(errors.Errorf("invalid hash %v", h), w)
		return
	}

	http.ServeFile(w, r, path.Join(os.TempDir(), "booster", h))
}

// Sync requests the patch from the sert of files in path to the set of files on the primary
// and applies it locally
func Sync(path string, primary string, w http.ResponseWriter, r *http.Request) {
	// determine new files, which is all files we have in decompressed form only
	err := gzip.DecompressAllIn(path)
	if err != nil {
		bark(err, w)
	}
	old := sorted(gzip.ListDecompressedOnly(path))

	resp, err := http.PostForm(primary+"/prepare_diff", url.Values{"old": {strings.Join(old, "\n")}})
	if err != nil {
		bark(err, w)
		return
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		bark(err, w)
		return
	}
	h := string(bodyBytes)

	err = wharf.Apply(primary+"/patch?hash="+h, path)
	if err != nil {
		bark(err, w)
		return
	}

	err = gzip.RecompressAllIn(path)
	if err != nil {
		bark(err, w)
		return
	}
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

// bark writes an error response
func bark(err error, w http.ResponseWriter) {
	w.WriteHeader(500)
	fmt.Fprintf(w, "Unexpected error: %v\n", err)
	log.Print(err)
}
