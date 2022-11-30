package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime"

	"github.com/chainguard-dev/acls-in-yaml/pkg/platform"
	"k8s.io/klog/v2"
)

//go:embed *.tmpl
var content embed.FS

type Server struct{}

func New() *Server {
	server := &Server{}
	return server
}

func (s *Server) Serve() error {
	http.HandleFunc("/", s.Root())
	http.HandleFunc("/submit", s.Submit())
	http.HandleFunc("/healthz", s.Healthz())

	listenAddr := fmt.Sprintf(":%s", os.Getenv("PORT"))
	if listenAddr == ":" {
		listenAddr = ":8080"
	}
	klog.Infof("listening on %s", listenAddr)
	return http.ListenAndServe(listenAddr, nil)
}

func (s *Server) error(w http.ResponseWriter, err error) {
	msg := err.Error()
	_, file, line, ok := runtime.Caller(0)
	if ok {
		msg = fmt.Sprintf("%s:%d: %v", file, line, err)
	}
	klog.Errorf(msg)
	http.Error(w, msg, 500)
}

func (s *Server) Root() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFS(content, "home.tmpl")

		if err != nil {
			s.error(w, err)
		}

		data := struct {
			Available []platform.Processor
		}{
			Available: platform.Available(),
		}

		if err := t.Execute(w, data); err != nil {
			s.error(w, err)
		}
	}
}

func (s *Server) Submit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Call ParseForm() to parse the raw query and update r.PostForm and r.Form.
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}

		kind := r.FormValue("kind")
		t, err := template.ParseFS(content, "home.tmpl")
		if err != nil {
			s.error(w, err)
		}

		data := struct {
			Kind string
		}{
			Kind: kind,
		}

		if err := t.Execute(w, data); err != nil {
			s.error(w, err)
		}
	}
}

// Healthz returns a dummy healthz page - it's always happy here!
func (s *Server) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
