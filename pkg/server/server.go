package server

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"runtime"

	"github.com/chainguard-dev/yacls/pkg/platform"
	"gopkg.in/yaml.v3"
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
	_, file, line, ok := runtime.Caller(1)
	if ok {
		msg = fmt.Sprintf("%s:%d: %v", file, line, err)
	}
	klog.Errorf(msg)
	http.Error(w, msg, 500)
}

func (s *Server) Root() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				s.error(w, err)
				return
			}
		}

		t, err := template.ParseFS(content, "home.tmpl")
		if err != nil {
			s.error(w, err)
			return
		}

		chosen := r.FormValue("kind")
		isProcess := r.FormValue("process")
		project := r.FormValue("project")

		var proc platform.Processor
		var desc platform.ProcessorDescription
		klog.Infof("chosen: %s", chosen)
		var output []byte

		if chosen != "" {
			proc, err = platform.New(chosen)
			if err != nil {
				s.error(w, err)
				return
			}
			desc = proc.Description()
		}

		if isProcess != "" {
			f, _, err := r.FormFile("file")
			if err != nil {
				s.error(w, err)
				return
			}

			a, err := proc.Process(platform.Config{
				Path:    "",
				Reader:  f,
				Project: project,
			})

			platform.FinalizeArtifact(a)
			output, err = yaml.Marshal(a)
			if err != nil {
				s.error(w, err)
			}
		}

		klog.Infof("desc:")
		data := struct {
			Available []platform.Processor
			Chosen    string
			Desc      platform.ProcessorDescription
			Output    []byte
		}{
			Available: platform.Available(),
			Chosen:    chosen,
			Desc:      desc,
			Output:    output,
		}

		if err := t.Execute(w, data); err != nil {
			s.error(w, err)
			return
		}
	}
}

// Healthz returns a dummy healthz page - it's always happy here!
func (s *Server) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}
