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

// hideRoles are roles which every org member has; this is hidden to remove output spam
var hideRoles = map[string]bool{
	"roles/billing.user":                 true,
	"roles/resourcemanager.folderViewer": true,
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

type gcpGroupMembership struct {
	Expanded bool
	Member   gcpMember     `yaml:"preferredMemberKey"`
	Roles    []gcpRoleType `yaml:"roles"`
}

type gcpMember struct {
	ID string `yaml:"id"`
}

type gcpRoleType struct {
	Name string `yaml:"name"`
}

type gcpMemberCache = map[string][]gcpGroupMembership

// NewGCPMemberCache returns a populated structure to be used for caching membership lookups.
func NewGCPMemberCache() gcpMemberCache {
	return map[string][]gcpGroupMembership{}
}

// expandGCPMembers expands groups into lists of users.
func expandGCPMembers(identity string, project string, cache gcpMemberCache) ([]gcpGroupMembership, error) {
	if cache[identity] != nil {
		return cache[identity], nil
	}

	_, id, _ := strings.Cut(identity, ":")
	if !strings.HasPrefix(identity, "group:") {
		member := gcpGroupMembership{Member: gcpMember{ID: id}}
		return []gcpGroupMembership{member}, nil
	}

	cmd := exec.Command("gcloud", "identity", "groups", "memberships", "list", fmt.Sprintf("--group-email=%s", id), fmt.Sprintf("--project=%s", project))
	klog.Infof("executing %s", cmd)
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	memberships := []gcpGroupMembership{}
	dec := yaml.NewDecoder(bytes.NewReader(stdout))
	for {
		var doc gcpGroupMembership
		if err := dec.Decode(&doc); err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("decode: %v", err)
			}
			break
		}
		doc.Expanded = true
		memberships = append(memberships, doc)
	}

	cache[identity] = memberships
	return memberships, nil
}

// GCP returns multiple roles for a group membership, but only the highest has any meaning.
func highestGCPRoleType(types []gcpRoleType) string {
	highest := ""

	// MEMBER -> ADMIN -> OWNER

	for _, roleType := range types {
		t := strings.ToUpper(roleType.Name)
		if highest == "" {
			highest = t
		}
		if highest == "MEMBER" {
			highest = t
		}

		if t == "OWNER" {
			highest = t
		}

		if t == "ADMIN" && highest == "MEMBER" {
			highest = t
		}
	}

	return highest
}

// GoogleCloudIAMPolicy uses gcloud to generate a list of GCP members.
func GoogleCloudIAMPolicy(project string, identityProject string, cache gcpMemberCache) (*Artifact, error) {
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
	if identityProject == "" {
		identityProject = project
	}

	seen := map[string]map[string]bool{}

	// YAML is parsed, lets figure out the users & roles
	for _, d := range docs {
		if a.Metadata.ID == "" {
			a.Metadata.ID = d.ID
			a.Metadata.Name = fmt.Sprintf("Google Cloud IAM Policy for %s", d.ID)
		}
		for _, b := range d.Policy.Bindings {
			for _, m := range b.Members {
				expanded, err := expandGCPMembers(m, identityProject, cache)
				if err != nil {
					return nil, fmt.Errorf("expand members %s: %w", m, err)
				}

				for _, e := range expanded {
					entity := e.Member.ID
					if users[entity] == nil {
						users[entity] = &User{Account: entity}
						seen[entity] = map[string]bool{}
					}
					if !seen[entity][b.Role] {
						if !hideRoles[b.Role] {
							users[entity].Roles = append(users[entity].Roles, b.Role)
							seen[entity][b.Role] = true
						}
					}

					if e.Expanded && !seen[entity][m] {
						_, id, _ := strings.Cut(m, ":")
						em := Membership{
							Name: id,
						}
						em.Role = highestGCPRoleType(e.Roles)

						// Shorten output
						if em.Role == "MEMBER" {
							em.Role = ""
						}
						users[entity].Groups = append(users[entity].Groups, em)
						seen[entity][m] = true
					}
				}
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

		if len(u.Roles) == 0 {
			klog.Infof("skipping %s (no important roles)", u.Account)
			continue
		}

		a.Users = append(a.Users, *u)
	}

	return a, nil
}
