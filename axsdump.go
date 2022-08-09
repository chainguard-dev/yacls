package main

import (
	"flag"
	"fmt"

	"chainguard.dev/axsdump/pkg/axs"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

var (
	googleWorkspaceAuditCSVFlag = flag.String("google-workspace-audit-csv", "", "Path to Google Workspace Audit CSV (delayed)")
	googleWorkspaceUsersCSVFlag = flag.String("google-workspace-users-csv", "", "Path to Google Workspace Users CSV (live)")
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

	for _, a := range artifacts {
		bs, err := yaml.Marshal(a)
		if err != nil {
			klog.Exitf("encode: %v", err)
		}
		fmt.Printf("ARTIFACT:\n%s\n", bs)
	}
}
