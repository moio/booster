package api

import (
	"fmt"
	"github.com/moio/booster/wharf"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func Diff(path string, w http.ResponseWriter, r *http.Request) {
	current := r.FormValue("current")
	acceptList := []string{}
	for _, f := range strings.Split(current, "\n") {
		acceptList = append(acceptList, filepath.Join(path, f))
	}
	filter := wharf.NewAcceptListFilter(acceptList)
	wharf.CreatePatch(path, filter.Filter, path, wharf.PreventClosing(w))
}

func Sync(path string, primary string, w http.ResponseWriter, r *http.Request) {
	current := []string{}
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		relative, _ := filepath.Rel(path, p)
		current = append(current, relative)
		return nil
	})

	resp, err := http.PostForm(primary+"/diff", url.Values{"current": {strings.Join(current, "\n")}})
	if err != nil {
		bark(err, w)
		return
	}

	patchFile, err := ioutil.TempFile(os.TempDir(), "patch-")
	if err != nil {
		bark(err, w)
		return
	}

	buf := make([]byte, 1024*1024*16)
	_, err = io.CopyBuffer(patchFile, resp.Body, buf)
	if err != nil {
		bark(err, w)
		return
	}
	err = patchFile.Close()
	if err != nil {
		bark(err, w)
		return
	}

	err = wharf.Apply(patchFile.Name(), path)
	if err != nil {
		bark(err, w)
		return
	}
}

func bark(err error, w http.ResponseWriter) {
	w.WriteHeader(500)
	fmt.Fprintf(w, "Unexpected error: %v", err)
	log.Print(err)
}
