package main

import (
	"bytes"
	"flag"
	"fmt"
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
	/* googleWorkspaceAuditCSVFlag = flag.String("google-audit-csv", "", fmt.Sprintf("Path to Google Workspace Audit CSV (delayed).\n%s", steps(platform.GoogleWorkspaceAuditSteps)))
	googleWorkspaceUsersCSVFlag = flag.String("google-users-csv", "", fmt.Sprintf("Path to Google Workspace Users CSV (live)\n%s", steps(platform.GoogleWorkspaceUsersSteps)))
	githubOrgMembersCSVFlag     = flag.String("github-org-csv", "", fmt.Sprintf("Path to Github Org Members CSV\n%s", steps(platform.GithubOrgSteps)))
	slackMembersCSVFlag         = flag.String("slack-csv", "", fmt.Sprintf("Path to Slack Members CSV\n%s", steps(platform.SlackSteps)))
	onePasswordFlag             = flag.String("1password-csv", "", fmt.Sprintf("Path to 1Password Team CSV\n%s", steps(platform.OnePasswordTeam{}.Description().Steps)))
	kolideUsersCSVFlag          = flag.String("kolide-csv", "", fmt.Sprintf("Path to Kolide Users CSV\n%s", steps(platform.KolideSteps)))
	vercelMembersHTMLFlag       = flag.String("vercel-html", "", fmt.Sprintf("Path to Vercel Members HTML\n%s", steps(platform.VercelSteps)))
	ghostStaffHTMLFlag          = flag.String("ghost-html", "", fmt.Sprintf("Path to Ghost Staff HTML\n%s", steps(platform.GhostSteps)))
	webflowMembersHTMLFlag      = flag.String("webflow-html", "", fmt.Sprintf("Path to Ghost Members HTML\n%s", steps(platform.WebflowSteps)))
	secureframePersonnelCSVFlag = flag.String("secureframe-csv", "", fmt.Sprintf("Path to Secureframe Personnel CSV\n%s", steps(platform.SecureframeSteps)))
	gcpIAMProjectsFlag          = flag.String("gcp-projects", "", "Comma-separated list of GCP projects to fetch IAM policies for")
	gcpIdentityProject          = flag.String("gcp-identity-project", "", "Optional GCP project for group resolution (requires cloudidentity API)")
	*/

	inputFlag   = flag.String("input", "", "path to input file")
	projectFlag = flag.String("project", "", "path to input file")
	kindFlag    = flag.String("kind", "", "kind of file to process")
	serveFlag   = flag.Bool("serve", false, "Enable server mode (web UI)")
	outDirFlag  = flag.String("out-dir", "", "output YAML files to this directory")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if *serveFlag {
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

	f, err := os.Open(*inputFlag)
	if err != nil {
		klog.Fatalf("unable to open: %v", err)
	}
	defer f.Close()

	a, err := p.Process(platform.Config{
		Path:    *inputFlag,
		Reader:  f,
		Project: *projectFlag,
	})

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
				name = a.Metadata.Kind + "-" + a.Metadata.ID + ".yaml"
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
