package api

import (
	"crypto/sha512"
	"fmt"
	"github.com/moio/booster/wharf"
	"github.com/pkg/errors"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Diff computes the patch between all files in the current registry and the list of files
// passed in the request body.
// The result is cached in a temporary directory by hash, returned in the response body
func Diff(basedir string, w http.ResponseWriter, r *http.Request) {
	current := r.FormValue("current")

	oldList := listFilesIn(basedir)

	newList := []string{}
	for _, f := range strings.Split(current, "\n") {
		newList = append(newList, filepath.Join(basedir, f))
	}

	h := hash(oldList, newList)

	os.MkdirAll(path.Join(os.TempDir(), "booster"), 0700)
	patchPath := path.Join(os.TempDir(), "booster", h)
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		f, err := os.Create(patchPath)
		if err != nil {
			bark(err, w)
			return
		}
		filter := wharf.NewAcceptListFilter(newList)
		wharf.CreatePatch(basedir, filter.Filter, basedir, wharf.PreventClosing(f))
		err = f.Close()
		if err != nil {
			bark(err, w)
			return
		}
	}

	io.WriteString(w, h)
}

// Patch serves a patch previously computed via Diff. It expects a hash value as parameter
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
	current := listFilesIn(path)

	resp, err := http.PostForm(primary+"/diff", url.Values{"current": {strings.Join(current, "\n")}})
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
}

// Lists all files in a directory
func listFilesIn(path string) []string {
	current := []string{}
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		relative, _ := filepath.Rel(path, p)
		current = append(current, relative)
		return nil
	})
	return current
}

// hash computes a hash from two lists of file paths
func hash(old []string, new []string) string {
	h := sha512.New()
	for _, f := range old {
		io.WriteString(h, f)
	}
	io.WriteString(h, "//////")
	for _, f := range new {
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
