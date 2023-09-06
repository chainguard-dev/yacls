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

	"github.com/chainguard-dev/yacls/v2/pkg/compare"
	"github.com/chainguard-dev/yacls/v2/pkg/platform"
	"github.com/chainguard-dev/yacls/v2/pkg/server"
	"github.com/gocarina/gocsv"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

func kindHelp() string {
	lines := []string{"\nDetailed steps for each kind:\n"}
	for _, k := range platform.Available() {
		d := k.Description()
		lines = append(lines, fmt.Sprintf("# %s\n", d.Name))
		for _, s := range d.Steps {
			lines = append(lines, fmt.Sprintf(" * %s", s))
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

var (
	inputFlag              = flag.String("input", "", "path to input file")
	compareFlag            = flag.String("compare", "", "path to file to compare against")
	projectFlag            = flag.String("project", "", "specific project to process within the kind")
	gcpIdentityProjectFlag = flag.String("gcp-identity-project", "", "project to use for GCP Cloud Identity lookups")
	kindFlag               = flag.String("kind", "", fmt.Sprintf("kind of input to process. valid values: \n  * %s\n%s", strings.Join(platform.AvailableKinds(), "\n  * "), kindHelp()))
	serveFlag              = flag.Bool("serve", false, "Enable server mode (web UI)")
	inDirFlag              = flag.String("in-dir", "", "process all input files found directly within this directory, guessing kinds")
	outDirFlag             = flag.String("out-dir", "", "output YAML files to this directory")
)

func main() {
	// Pollutes --help with flags no one will need
	klog.InitFlags(nil)
	flag.Parse()

	if *serveFlag || os.Getenv("SERVE_MODE") == "1" {
		s := server.New()
		if err := s.Serve(); err != nil {
			log.Fatalf("serve failed: %v", err)
		}
		os.Exit(0)
	}

	if *compareFlag != "" {
		changes := []compare.Change{}

		if *inDirFlag == "" {
			cs, err := compareSummary(*inputFlag, *compareFlag)
			if err != nil {
				log.Fatalf("compare failed: %v", err)
			}
			changes = append(changes, cs...)
		} else {
			files, err := os.ReadDir(*inDirFlag)
			if err != nil {
				log.Fatal(err)
			}

			for _, file := range files {
				if file.IsDir() {
					continue
				}
				src := filepath.Join(*inDirFlag, file.Name())
				dest := filepath.Join(*compareFlag, file.Name())
				cs, err := compareSummary(src, dest)
				if err != nil {
					log.Fatalf("compare failed: %v", err)
				}
				changes = append(changes, cs...)
			}
		}
		s, err := gocsv.MarshalString(&changes)
		if err != nil {
			log.Fatalf("marshal: %v", err)
		}
		fmt.Println(s)
		os.Exit(0)
	}

	generate()
}

func compareSummary(fromPath string, toPath string) ([]compare.Change, error) {
	cs := []compare.Change{}
	bs, err := os.ReadFile(fromPath)
	if err != nil {
		return cs, fmt.Errorf("read: %w", err)
	}

	from := platform.Artifact{}
	if err := yaml.Unmarshal(bs, &from); err != nil {
		return cs, fmt.Errorf("unmarshal: %w", err)
	}

	bs, err = os.ReadFile(toPath)
	if err != nil {
		return cs, fmt.Errorf("read: %w", err)
	}

	to := platform.Artifact{}
	if err := yaml.Unmarshal(bs, &to); err != nil {
		return cs, fmt.Errorf("unmarshal: %w", err)
	}

	return compare.Summary(from, to)
}

// generate is the common path for generating and outputting YAML
func generate() {
	inputs := []string{}
	if *inputFlag != "" {
		inputs = append(inputs, *inputFlag)
	}
	if *inDirFlag != "" {
		files, err := os.ReadDir(*inDirFlag)
		if err != nil {
			log.Fatal(err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			log.Printf("found input file: %s", file.Name())
			inputs = append(inputs, filepath.Join(*inDirFlag, file.Name()))
		}
	}

	// these workflows don't require an input
	if strings.HasPrefix(*kindFlag, "gcp") {
		inputs = append(inputs, "")
	}

	if len(inputs) == 0 && *kindFlag == "" {
		log.Fatalf("found no inputs or kind flag to work with")
	}

	gcpMemberCache := platform.NewGCPMemberCache()
	artifacts := []*platform.Artifact{}
	var err error

	for _, i := range inputs {
		kind := *kindFlag
		if kind == "" {
			kind, err = platform.SuggestKind(i)
			if err != nil {
				log.Fatalf("suggest kind: %v", err)
				continue
			}
		}

		klog.Infof("kind: %q", kind)
		var f io.ReadCloser
		p, err := platform.New(kind)
		if err != nil {
			klog.Fatalf("unable to create %q platform: %v", *kindFlag, err)
		}

		if i != "" {
			f, err = os.Open(i)
			if err != nil {
				klog.Fatalf("unable to open: %v", err)
			}
			defer f.Close()
		}

		a, err := p.Process(platform.Config{
			Path:               i,
			Reader:             f,
			Project:            *projectFlag,
			Kind:               kind,
			GCPIdentityProject: *gcpIdentityProjectFlag,
			GCPMemberCache:     gcpMemberCache,
		})
		if err != nil {
			klog.Fatalf("process failed: %v", err)
		}

		artifacts = append(artifacts, a)
	}

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
