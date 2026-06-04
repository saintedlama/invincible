package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/saintedlama/invincible/internal/supervisor"
)

type supervisorIface interface {
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	RestartAll()
	Status() []supervisor.ProcessStatus
	Logs(name string, n int) []supervisor.LogEntry
}

type Server struct {
	sup      supervisorIface
	listener net.Listener
	router   chi.Router
}

// New creates a Server and binds a listener on 127.0.0.1 at the port extracted
// from addr (default 7777). If that port is already in use, an ephemeral port
// is chosen automatically.
func New(sup supervisorIface, addr string) (*Server, error) {
	port := "7777"
	if addr != "" {
		if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
			port = p
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		// Preferred port in use — let the OS pick a free one.
		l, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("api: could not bind listener: %w", err)
		}
	}
	s := &Server{sup: sup, listener: l}
	s.router = s.buildRouter()
	return s, nil
}

// Addr returns the address the server is actually listening on, e.g. "127.0.0.1:7777".
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

func (s *Server) ListenAndServe() error {
	return http.Serve(s.listener, s.router)
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleIndex)
	r.Get("/openapi.json", s.handleSpec)

	r.Route("/processes", func(r chi.Router) {
		r.Get("/", s.listProcesses)
		r.Post("/restart-all", s.restartAllProcesses)
		r.Route("/{name}", func(r chi.Router) {
			r.Get("/", s.getProcess)
			r.Get("/logs", s.getLogs)
			r.Post("/start", s.startProcess)
			r.Post("/stop", s.stopProcess)
			r.Post("/restart", s.restartProcess)
		})
	})

	return r
}

func (s *Server) listProcesses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.sup.Status())
}

func (s *Server) getProcess(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	for _, st := range s.sup.Status() {
		if st.Name == name {
			writeJSON(w, st)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	n := 100
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil {
			n = v
		}
	}
	entries := s.sup.Logs(name, n)
	if r.URL.Query().Get("format") == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, e := range entries {
			fmt.Fprintln(w, e.Line)
		}
		return
	}
	writeJSON(w, entries)
}

func (s *Server) startProcess(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, s.sup.Start)
}

func (s *Server) stopProcess(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, s.sup.Stop)
}

func (s *Server) restartProcess(w http.ResponseWriter, r *http.Request) {
	s.action(w, r, s.sup.Restart)
}

func (s *Server) restartAllProcesses(w http.ResponseWriter, r *http.Request) {
	s.sup.RestartAll()
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) action(w http.ResponseWriter, r *http.Request, fn func(string) error) {
	name := chi.URLParam(r, "name")
	if err := fn(name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buildSpec()) //nolint
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Invincible</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 36rem; margin: 2rem auto; padding: 0 1rem; color: #ccc; background: #111; }
  h1 { color: #fff; font-size: 1.5rem; }
  a { color: #7af; text-decoration: none; }
  a:hover { text-decoration: underline; }
  ul { padding-left: 1.2rem; }
  li { margin: 0.4rem 0; }
  code { background: #222; padding: 0.1em 0.3em; border-radius: 3px; }
</style>
</head>
<body>
<h1>Invincible</h1>
<p>Process manager HTTP API</p>
<ul>
  <li><a href="/openapi.json">OpenAPI spec</a> <code>/openapi.json</code></li>
  <li><a href="/processes">Processes</a> <code>GET /processes</code></li>
  <li><a href="/processes/restart-all">Restart all</a> <code>POST /processes/restart-all</code></li>
</ul>
<p>Per-process endpoints: <code>/processes/{name}</code>, <code>/processes/{name}/logs</code>, <code>/processes/{name}/start</code>, <code>/processes/{name}/stop</code>, <code>/processes/{name}/restart</code></p>
</body>
</html>`))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint
}
