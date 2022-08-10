package axs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

var gcpOrgSteps = []string{
	"Execute 'axsdump --gcloud-iam-projects=[project,...]'",
}

type ancestorsIAMPolicyDoc struct {
	ID     string `yaml:"id"`
	Type   string `yaml:"type"`
	Policy policy `yaml:"policy"`
}

type policy struct {
	Bindings []binding `yaml:"bindings"`
}

type binding struct {
	Members []string `yaml:"members"`
	Role    string   `yaml:"role"`
}

// GoogleCloudIAMPolicy uses gcloud to generate a list of GCP members.
func GoogleCloudIAMPolicy(project string, identityProject string) (*Artifact, error) {
	cmd := exec.Command("gcloud", "projects", "get-ancestors-iam-policy", project)
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	klog.Infof("output: %s", stdout)

	docs := []ancestorsIAMPolicyDoc{}
	dec := yaml.NewDecoder(bytes.NewReader(stdout))
	for {
		var doc ancestorsIAMPolicyDoc
		if err := dec.Decode(&doc); err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("decode: %v", err)
			}
			break
		}
		docs = append(docs, doc)
	}

	cu, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("user: %w", err)
	}

	a := &Artifact{
		Metadata: &Source{
			Kind:        "gcp_iam_policy",
			GeneratedAt: time.Now(),
			GeneratedBy: cu.Username,
			SourceDate:  time.Now().Format(SourceDateFormat),
			Process:     gcpOrgSteps,
		},
	}

	users := map[string]*User{}

	// YAML is parsed, lets figure out the users & roles
	for _, d := range docs {
		if a.Metadata.ID == "" {
			a.Metadata.ID = d.ID
			a.Metadata.Name = fmt.Sprintf("Google Cloud IAM Policy for %s", d.ID)
		}
		for _, b := range d.Policy.Bindings {
			for _, m := range b.Members {
				if users[m] == nil {
					users[m] = &User{Account: m}
				}
				users[m].Roles = append(users[m].Roles, b.Role)
			}
		}
	}

	for _, u := range users {
		if strings.HasPrefix(u.Account, "domain:") {
			continue
		}

		if strings.HasPrefix(u.Account, "serviceAccount:") {
			a.Bots = append(a.Bots, *u)
			continue
		}
		a.Users = append(a.Users, *u)
	}

	return a, nil
}
