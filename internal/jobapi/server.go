/*
the runner side of the job API, Start creates a private temp folder, opens a unix
socket in it and serves HTTP on that socker in the background. each endpoint
from types.go gets a handler that connects a request to the code we already have
*/
package jobapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"qahyacli/internal/config"
	"qahyacli/internal/state"
)


type Server struct {
	store 		*state.Store
	dir 		string
	sock 		string
	http 		*http.Server
}

// Start opens the job's socker and server the API in the background
func Start(store *state.Store) (*Server, error) {
	// unix socket paths are limited to ~104 bytes on macOs, and macOS $TMPDIR (/var/folders/...)
	// is long enough to hit that. /tmp is short. MkdirTemp creates the dire 0700, so only
	// this user can reach the socket

	dir, err := os.MkdirTemp("/tmp", "qahya-")
	if err != nil {
		return nil, fmt.Errorf("create socker dir: %w", err)
	}
	sock := filepath.Join(dir, "job.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("listen on %s: %w", sock, err)
	}

	s := &Server{store: store, dir: dir, sock: sock}

	// Go 1.22+ patterns: method + path, {name} read with r.PathValue
	mux := http.NewServeMux()
	mux.HandleFunc("GET /env", s.handleList)
	mux.HandleFunc("GET /env/{name}", s.handleGet)
	mux.HandleFunc("PUT /env/{name}", s.handleSet)
	mux.HandleFunc("DELETE /env/{name}", s.handleUnset)
	mux.HandleFunc("POST /load", s.handleLoad)

	s.http = &http.Server{Handler: mux}
	// Serve returns http.ErrServerClosed after Close; nothing to report then

	go s.http.Serve(ln)

	return s, nil
}

func (s *Server) SocketPath() string { return s.sock }

// Close waits briefly for in-flight requests (a step may have started a qahyacli env set), then
// removes the socket dir, so no stale socket survives the job
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := s.http.Shutdown(ctx)
	os.RemoveAll(s.dir)
	return err
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	snap := s.store.Snapshot()
	out := make([]Entry, 0, len(snap))
	for _, name := range slices.Sorted(maps.Keys(snap)) {
		out = append(out, toWire(name, snap[name]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	e, ok := s.store.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("%s is not set", name))
		return
	}

	v, err := e.Resolve()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := toWire(name, e)
	out.Value = v
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := state.ValidateName(name); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	
	var req setRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.store.Set(name, state.Entry{Value: req.Value, Source: fmt.Sprintf("step %q", step(r))})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := state.ValidateName(name); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.store.Delete(name)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleLoad(w http.ResponseWriter, r *http.Request) {
	var req loadRequest
	if err := decode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !filepath.IsAbs(req.Path) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("path must be absolute, got %q", req.Path))
		return
	}

	entries, err := config.Load(req.Path, req.Profile)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	out := make([]Entry, 0, len(entries))
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		e := entries[name]
		e.Source += fmt.Sprintf(" via step %q", step(r))
		entries[name] = e

		we := toWire(name, e)
		if req.Export {
			// the client is about to put these into its own shell, so it needs
			// the actual values, secrets included
			v, err := e.Resolve()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			we.Value = v
		}
		out = append(out, we)
	}

	// applied only after everything above succeeded: a failed load changes nothing
	s.store.SetMany(entries)
	writeJSON(w, http.StatusOK, loadResponse{Entries: out})
}

// toWire never includes a secret's value; handlers that need it resolve and fill it in explicitly
func toWire(name string, e state.Entry) Entry {
	out := Entry{Name: name, Source: e.Source}
	if e.IsSecret() {
		out.Secret = true
		out.Ref = e.Secret.String()
	} else {
		out.Value = e.Value
	}
	return out
}


func step(r *http.Request) string {
	if s := r.Header.Get(stepHeader); s != "" {
		return s
	}
	return "unknown"
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("bad request body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New(http.StatusText(status))
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}
