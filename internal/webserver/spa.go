// Package webserver serves a built single-page app from an embedded
// filesystem, falling back to index.html for client-side routes (e.g.
// /board, /units/:id) so a full page load or refresh works correctly.
package webserver

import (
	"io/fs"
	"net/http"
	"path"
)

// SPAHandler serves files from root (typically an fs.Sub of an embed.FS
// rooted at "dist"). Requests for a path that doesn't exist as a file are
// served index.html instead, so client-side routing can take over.
func SPAHandler(root fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(root))

	return func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if p != "/" {
			if _, err := fs.Stat(root, p[1:]); err != nil {
				r = cloneWithPath(r, "/")
			}
		}
		fileServer.ServeHTTP(w, r)
	}
}

func cloneWithPath(r *http.Request, newPath string) *http.Request {
	r2 := r.Clone(r.Context())
	r2.URL.Path = newPath
	return r2
}
