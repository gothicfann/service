package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/dimfeld/httptreemux/v5"
)

func API(build string, shutdown chan os.Signal, log *log.Logger) http.Handler {
	tm := httptreemux.NewContextMux()

	h := func(w http.ResponseWriter, r *http.Request) {
		status := struct {
			Status string
		}{
			Status: "OK",
		}
		json.NewEncoder(w).Encode(status)
	}

	tm.Handle(http.MethodGet, "/test", h)
	return tm
}
