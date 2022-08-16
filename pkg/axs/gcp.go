package axs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"os/user"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

var gcpOrgSteps = []string{
	"Execute 'axsdump --gcloud-iam-projects=[project,...]'",
}

// hideRoles are roles which every org member has; this is hidden to remove output spam.
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
	klog.Infof("output: %s", stdout)

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

	// CONFUSION ALERT!@$!@$!@
	// Google Groups have a single role (owner, member, admin) - we refer to those as "role"
	// GCP has multiple roles (role/viewer, etc) - we refer to those as "permissions" to avoid confusion

	seenInGrp := map[string]map[string]bool{}
	seenWithPerm := map[string]map[string]bool{}

	groups := map[string]*Group{}

	// YAML is parsed, lets figure out the users & roles
	for _, d := range docs {
		if a.Metadata.ID == "" {
			a.Metadata.ID = d.ID
			a.Metadata.Name = fmt.Sprintf("Google Cloud IAM Policy for %s", d.ID)
		}

		// Bindings are a relationship of roles to members
		for _, binding := range d.Policy.Bindings {
			// bindMembers may be individuals or groups
			for _, bindMember := range binding.Members {
				// Expand all groups into individuals
				expanded, err := expandGCPMembers(bindMember, identityProject, cache)
				if err != nil {
					return nil, fmt.Errorf("expand members %s: %w", bindMember, err)
				}
				klog.Infof("m: %s -- role %s / members %s / expanded: %s", bindMember, binding.Role, binding.Members, expanded)

				for _, membership := range expanded {
					// id is the individuals login
					id := membership.Member.ID
					if users[id] == nil {
						klog.Infof("new user: %s", id)
						users[id] = &User{Account: id}
						seenWithPerm[id] = map[string]bool{}
						seenInGrp[id] = map[string]bool{}
					}

					perm := binding.Role
					if hideRoles[perm] {
						klog.Infof("%s: hiding built-in hidden role: %s", id, perm)
						continue
					}

					if !membership.Expanded && !seenWithPerm[id][perm] {
						users[id].Permissions = append(users[id].Permissions, perm)
						seenWithPerm[id][perm] = true
					}

					// Everything after this deals with expanded groups
					if !membership.Expanded {
						continue
					}

					_, grp, _ := strings.Cut(bindMember, ":")

					if groups[grp] == nil {
						klog.Infof("new group: %s with members: %s - permission: %s", grp, expanded, perm)
						groups[grp] = &Group{Name: grp}
						seenWithPerm[grp] = map[string]bool{}
					}

					if !seenWithPerm[grp][perm] {
						groups[grp].Permissions = append(groups[grp].Permissions, perm)
						seenWithPerm[grp][perm] = true
					}

					highestRole := highestGCPRoleType(membership.Roles)
					if highestRole == "MEMBER" {
						highestRole = ""
					}

					if !seenInGrp[id][grp] {
						groups[grp].Members = append(groups[grp].Members, id)
						users[id].Groups = append(users[id].Groups, Membership{Name: grp, Role: highestRole})
						seenInGrp[id][grp] = true
					}
				}
			}
		}
	}

	for _, g := range groups {
		sort.Strings(g.Members)
		sort.Strings(g.Permissions)

		for _, m := range g.Members {
			klog.Infof("group %s - member %s", g.Name, m)
			klog.Infof("member details: %+v", users[m])

			sort.Slice(users[m].Groups, func(i, j int) bool {
				return users[m].Groups[i].Name < users[m].Groups[j].Name
			})

			for i, ug := range users[m].Groups {
				if ug.Name == g.Name {
					users[m].Groups[i].Permissions = g.Permissions
					users[m].Groups[i].Description = g.Description
				}
			}
		}
		a.Groups = append(a.Groups, *g)
	}

	for _, u := range users {
		if strings.HasPrefix(u.Account, "domain:") {
			continue
		}

		if strings.HasSuffix(u.Account, "gserviceaccount.com") {
			a.Bots = append(a.Bots, *u)
			continue
		}

		sort.Slice(a.Groups, func(i, j int) bool {
			return a.Groups[i].Name < a.Groups[j].Name
		})

		// Hide redundant permissions from the user view
		seenPerm := map[string]bool{}
		for i, g := range u.Groups {
			showPerms := []string{}
			for _, p := range g.Permissions {
				if seenPerm[p] {
					klog.Infof("dropping dupe permission %s in %s:%s", p, u.Account, g.Name)
					continue
				}
				showPerms = append(showPerms, p)
				seenPerm[p] = true
			}
			u.Groups[i].Permissions = showPerms
		}

		effectivePerms := []string{}
		effectivePerms = append(effectivePerms, u.Permissions...)
		for _, g := range u.Groups {
			effectivePerms = append(effectivePerms, g.Permissions...)
		}

		if len(effectivePerms) == 0 {
			klog.Infof("skipping %s (no important roles)", u.Account)
			continue
		}

		a.Users = append(a.Users, *u)
	}

	return a, nil
}
