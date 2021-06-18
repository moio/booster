package api

import (
	"github.com/moio/regsync/wharf"
	"net/http"
	"strings"
)

func Diff(path string, w http.ResponseWriter, r *http.Request) {
	current := r.FormValue("current")
	filter := wharf.NewAcceptListFilter(strings.Split(current, "\n"))
	wharf.CreatePatch(path, filter.Filter, path, wharf.PreventClosing(w))
}