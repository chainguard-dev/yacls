package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainguard-dev/acls-in-yaml/pkg/platform"
	"github.com/chainguard-dev/acls-in-yaml/pkg/server"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

func steps(s []string) string {
	// omit the last step if it mentions acls-in-yaml
	if strings.Contains(s[len(s)-1], "acls-in-yaml") {
		s = s[0 : len(s)-1]
	}

	return fmt.Sprintf("Steps:\n  * %s", strings.Join(s, "\n  * "))
}

var (
	inputFlag              = flag.String("input", "", "path to input file")
	projectFlag            = flag.String("project", "", "specific project to process within the kind")
	gcpIdentityProjectFlag = flag.String("gcp-identity-project", "", "project to use for GCP Cloud Identity lookups")
	kindFlag               = flag.String("kind", "", fmt.Sprintf("kind of input to process. Valid values: \n  * %s", strings.Join(platform.AvailableKinds(), "\n  * ")))
	serveFlag              = flag.Bool("serve", false, "Enable server mode (web UI)")
	outDirFlag             = flag.String("out-dir", "", "output YAML files to this directory")
)

func main() {
	// Pollutes --help with flags no one will need
	// klog.InitFlags(nil)
	flag.Parse()

	if *serveFlag || os.Getenv("SERVE_MODE") == "1" {
		s := server.New()
		if err := s.Serve(); err != nil {
			log.Fatalf("serve failed: %v", err)
		}
		os.Exit(0)
	}

	p, err := platform.New(*kindFlag)
	if err != nil {
		klog.Fatalf("unable to create %q platform: %v", *kindFlag, err)
	}

	var f io.ReadCloser
	if *inputFlag != "" {
		f, err = os.Open(*inputFlag)
		if err != nil {
			klog.Fatalf("unable to open: %v", err)
		}
		defer f.Close()
	}

	gcpMemberCache := platform.NewGCPMemberCache()

	a, err := p.Process(platform.Config{
		Path:               *inputFlag,
		Reader:             f,
		Project:            *projectFlag,
		Kind:               *kindFlag,
		GCPIdentityProject: *gcpIdentityProjectFlag,
		GCPMemberCache:     gcpMemberCache,
	})
	if err != nil {
		klog.Fatalf("process failed: %v", err)
	}

	artifacts := []*platform.Artifact{a}

	for _, a := range artifacts {
		platform.FinalizeArtifact(a)

		bs, err := yaml.Marshal(a)
		if err != nil {
			klog.Exitf("encode: %v", err)
		}

		// Improve readability by adding a newline before each account
		bs = bytes.ReplaceAll(bs, []byte("    - account"), []byte("\n    - account"))
		// Remove the first double newline
		bs = bytes.Replace(bs, []byte("\n\n"), []byte("\n"), 1)

		if *outDirFlag != "" {
			name := a.Metadata.Kind + ".yaml"
			if a.Metadata.ID != "" {
				name = a.Metadata.Kind + "_" + a.Metadata.ID + ".yaml"
			}

			outPath := filepath.Join(*outDirFlag, name)
			err := os.WriteFile(outPath, bs, 0o600)
			if err != nil {
				klog.Exitf("writefile: %s", err)
			}
			klog.Infof("wrote to %s (%d bytes)", outPath, len(bs))
		} else {
			fmt.Printf("---\n%s\n", bs)
		}
	}
}
