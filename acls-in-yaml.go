package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainguard-dev/acls-in-yaml/pkg/axs"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

func steps(s []string) string {
	// omit the last step if it mentions axsdump
	if strings.Contains(s[len(s)-1], "axsdump") {
		s = s[0 : len(s)-1]
	}

	return fmt.Sprintf("Steps:\n  * %s", strings.Join(s, "\n  * "))
}

var (
	googleWorkspaceAuditCSVFlag = flag.String("google-audit-csv", "", fmt.Sprintf("Path to Google Workspace Audit CSV (delayed).\n%s", steps(axs.GoogleWorkspaceAuditSteps)))
	googleWorkspaceUsersCSVFlag = flag.String("google-users-csv", "", fmt.Sprintf("Path to Google Workspace Users CSV (live)\n%s", steps(axs.GoogleWorkspaceUsersSteps)))
	githubOrgMembersCSVFlag     = flag.String("github-org-csv", "", fmt.Sprintf("Path to Github Org Members CSV\n%s", steps(axs.GithubOrgSteps)))
	slackMembersCSVFlag         = flag.String("slack-csv", "", fmt.Sprintf("Path to Slack Members CSV\n%s", steps(axs.SlackSteps)))
	kolideUsersCSVFlag          = flag.String("kolide-csv", "", fmt.Sprintf("Path to Kolide Users CSV\n%s", steps(axs.KolideSteps)))
	vercelMembersHTMLFlag       = flag.String("vercel-html", "", fmt.Sprintf("Path to Vercel Members HTML\n%s", steps(axs.VercelSteps)))
	ghostStaffHTMLFlag          = flag.String("ghost-html", "", fmt.Sprintf("Path to Ghost Staff HTML\n%s", steps(axs.GhostSteps)))
	webflowMembersHTMLFlag      = flag.String("webflow-html", "", fmt.Sprintf("Path to Ghost Members HTML\n%s", steps(axs.WebflowSteps)))
	secureframePersonnelCSVFlag = flag.String("secureframe-csv", "", fmt.Sprintf("Path to Secureframe Personnel CSV\n%s", steps(axs.SecureframeSteps)))
	gcpIAMProjectsFlag          = flag.String("gcp-projects", "", "Comma-separated list of GCP projects to fetch IAM policies for")
	gcpIdentityProject          = flag.String("gcp-identity-project", "", "Optional GCP project for group resolution (requires cloudidentity API)")
	outDirFlag                  = flag.String("out-dir", "", "output YAML files to this directory")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	artifacts := []*axs.Artifact{}

	if *googleWorkspaceAuditCSVFlag != "" {
		a, err := axs.GoogleWorkspaceAudit(*googleWorkspaceAuditCSVFlag)
		if err != nil {
			klog.Exitf("google workspace audit: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *googleWorkspaceUsersCSVFlag != "" {
		a, err := axs.GoogleWorkspaceUsers(*googleWorkspaceUsersCSVFlag)
		if err != nil {
			klog.Exitf("google workspace users: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *githubOrgMembersCSVFlag != "" {
		a, err := axs.GithubOrgMembers(*githubOrgMembersCSVFlag)
		if err != nil {
			klog.Exitf("github org members: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *slackMembersCSVFlag != "" {
		a, err := axs.SlackMembers(*slackMembersCSVFlag)
		if err != nil {
			klog.Exitf("slack members: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *kolideUsersCSVFlag != "" {
		a, err := axs.KolideUsers(*kolideUsersCSVFlag)
		if err != nil {
			klog.Exitf("kolide users: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *vercelMembersHTMLFlag != "" {
		a, err := axs.VercelMembers(*vercelMembersHTMLFlag)
		if err != nil {
			klog.Exitf("vercel users: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *webflowMembersHTMLFlag != "" {
		a, err := axs.WebflowMembers(*webflowMembersHTMLFlag)
		if err != nil {
			klog.Exitf("webflow users: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *ghostStaffHTMLFlag != "" {
		a, err := axs.GhostStaff(*ghostStaffHTMLFlag)
		if err != nil {
			klog.Exitf("ghost staff: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *secureframePersonnelCSVFlag != "" {
		a, err := axs.SecureframePersonnel(*secureframePersonnelCSVFlag)
		if err != nil {
			klog.Exitf("secureframe personnel: %v", err)
		}

		artifacts = append(artifacts, a)
	}

	if *gcpIAMProjectsFlag != "" {
		cache := axs.NewGCPMemberCache()
		projects := strings.Split(*gcpIAMProjectsFlag, ",")
		for _, p := range projects {
			a, err := axs.GoogleCloudIAMPolicy(p, *gcpIdentityProject, cache)
			if err != nil {
				klog.Exitf("gcp iam: %v", err)
			}

			artifacts = append(artifacts, a)
		}
	}

	for _, a := range artifacts {
		axs.FinalizeArtifact(a)

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
