package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"chainguard.dev/axsdump/pkg/axs"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

var (
	googleWorkspaceAuditCSVFlag = flag.String("google-workspace-audit-csv", "", "Path to Google Workspace Audit CSV (delayed)")
	googleWorkspaceUsersCSVFlag = flag.String("google-workspace-users-csv", "", "Path to Google Workspace Users CSV (live)")
	githubOrgMembersCSVFlag     = flag.String("github-org-members-csv", "", "Path to Github Org Members CSV")
	slackMembersCSVFlag         = flag.String("slack-members-csv", "", "Path to Slack Members CSV")
	kolideUsersCSVFlag          = flag.String("kolide-users-csv", "", "Path to Kolide Users CSV")
	vercelMembersHTMLFlag       = flag.String("vercel-members-html", "", "Path to Vercel Members HTML")
	ghostStaffHTMLFlag          = flag.String("ghost-staff-html", "", "Path to Ghost Staff HTML")
	secureframePersonnelCSVFlag = flag.String("secureframe-personnel-csv", "", "Path to Secureframe Personnel CSV")

	outDirFlag = flag.String("out-dir", "", "output YAML files to this directory")
)

func main() {
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

	for _, a := range artifacts {
		axs.FinalizeArtifact(a)

		bs, err := yaml.Marshal(a)
		if err != nil {
			klog.Exitf("encode: %v", err)
		}

		if *outDirFlag != "" {
			outPath := filepath.Join(*outDirFlag, a.Metadata.Kind+".yaml")
			err := os.WriteFile(outPath, bs, 0o600)
			if err != nil {
				klog.Exitf("writefile: %w", err)
			}
			klog.Infof("wrote to %s (%d bytes)", outPath, len(bs))
		} else {
			fmt.Printf("---\n%s\n", bs)
		}
	}
}
