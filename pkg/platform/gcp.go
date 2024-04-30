package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

// hideRoles that have no impact for project visibility, or are internal to GCP
var hideRoles = map[string]bool{
	"roles/billing.user":                       true,
	"roles/billing.viewer":                     true,
	"roles/billing.creator":                    true,
	"roles/resourcemanager.folderViewer":       true,
	"roles/resourcemanager.projectCreator":     true,
	"roles/recommender.exporter":               true,
	"roles/billing.costsManager":               true,
	"roles/resourcemanager.organizationViewer": true,
	"roles/project.Creator":                    true,
	"roles/dlp.orgdriver":                      true,
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

type GCPMemberCache = map[string][]gcpGroupMembership

// NewGCPMemberCache returns a populated structure to be used for caching membership lookups.
func NewGCPMemberCache() GCPMemberCache {
	return map[string][]gcpGroupMembership{}
}

type gcpIdentity struct {
	Kind        string
	Domain      string
	Username    string
	Email       string
	DisplayName string
	Disabled    bool
	Deleted     bool

	IsServiceAccount bool
}

// parse <kind>:<name>@<domain>
func parseGCPIdentity(s string) gcpIdentity {
	kind := "unknown"
	id := s

	deleted := false
	if strings.HasPrefix(s, "deleted:") {
		deleted = true
	}

	if x := strings.LastIndex(s, ":"); x > 0 {
		kind = s[:x]
		id = s[x+1:]
	}

	// extra annotation for deleted users
	id, _, _ = strings.Cut(id, "?uid=")
	name, domain, _ := strings.Cut(id, "@")

	if strings.HasSuffix(domain, "gserviceaccount.com") {
		kind = "serviceAccount"
	}

	return gcpIdentity{
		Kind:     kind,
		Domain:   domain,
		Email:    fmt.Sprintf("%s@%s", name, domain),
		Username: name,
		Deleted:  deleted,
	}
}

type serviceAccount struct {
	Disabled    bool
	DisplayName string
	Email       string
}

type organization struct {
	DisplayName string
}

type projectInfo struct {
	ProjectNumber string `json:"projectNumber"`
	ProjectID     string `json:"projectID"`
}

func organizationsList() ([]string, error) {
	cmd := exec.Command("gcloud", "organizations", "list", "--format=json")
	klog.Infof("executing %s", cmd)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	klog.Infof("output: %s", stdout)

	orgs := []organization{}
	err = json.Unmarshal(stdout, &orgs)
	if err != nil {
		return nil, err
	}

	os := []string{}
	for _, s := range orgs {
		os = append(os, s.DisplayName)
	}

	return os, nil
}

func projectNumber(project string) (string, error) {
	cmd := exec.Command("gcloud", "projects", "describe", project, "--format=json")
	klog.Infof("executing %s", cmd)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	klog.Infof("output: %s", stdout)

	p := projectInfo{}
	err = json.Unmarshal(stdout, &p)
	if err != nil {
		return "", err
	}

	return p.ProjectNumber, nil
}

func projectsByNumber() (map[string]string, error) {
	cmd := exec.Command("gcloud", "projects", "list", "--format=json")
	klog.Infof("executing %s", cmd)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	klog.Infof("output: %s", stdout)

	ps := []projectInfo{}
	err = json.Unmarshal(stdout, &ps)
	if err != nil {
		return nil, err
	}

	pbn := map[string]string{}
	for _, p := range ps {
		pbn[p.ProjectNumber] = p.ProjectID
	}

	return pbn, nil
}

func serviceAccountList(project string) (map[string]gcpIdentity, error) {
	cmd := exec.Command("gcloud", "iam", "service-accounts", "list", "--format=json", fmt.Sprintf("--project=%s", project))
	klog.Infof("executing %s", cmd)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	klog.Infof("output: %s", stdout)

	sas := []serviceAccount{}
	err = json.Unmarshal(stdout, &sas)
	if err != nil {
		return nil, err
	}

	gids := map[string]gcpIdentity{}
	for _, s := range sas {
		klog.Infof("service account: %+v", s)
		gid := parseGCPIdentity(fmt.Sprintf("serviceAccount:%s", s.Email))
		gid.DisplayName = strings.TrimSpace(s.DisplayName)
		gid.Disabled = s.Disabled
		gids[s.Email] = gid
	}

	return gids, nil
}

// expandGCPMembers expands groups into lists of users.
func expandGCPMembers(identity string, project string, cache GCPMemberCache) ([]gcpGroupMembership, error) {
	if cache[identity] != nil {
		return cache[identity], nil
	}

	gid := parseGCPIdentity(identity)
	klog.Infof("might expand %+v", gid)

	// no expansion required
	if gid.Kind == "user" || gid.Kind == "serviceAccount" {
		member := gcpGroupMembership{Member: gcpMember{ID: gid.Email}}
		return []gcpGroupMembership{member}, nil
	}

	cmd := exec.Command("gcloud", "identity", "groups", "memberships", "list", fmt.Sprintf("--group-email=%s", gid.Email), fmt.Sprintf("--project=%s", project))
	klog.Infof("executing %s", cmd)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
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

type gcpRole struct {
	Name        string `yaml:",omitempty"`
	Title       string `yaml:",omitempty"`
	Description string `yaml:"description"`
}

func (gr *gcpRole) String() string {
	id := strings.ReplaceAll(gr.Name, "roles/", "")
	desc, _, _ := strings.Cut(gr.Description, ".")
	if desc == "" {
		desc = gr.Title
	}

	desc = strings.Replace(desc, "Access to ", "", 1)
	desc = strings.Replace(desc, "Read-only ", "read ", 1)
	desc = strings.Replace(desc, "Read only ", "read ", 1)
	desc = strings.Replace(desc, "Create and manage ", "Manage ", 1)
	desc = strings.Replace(desc, "The permission to ", "", 1)
	desc = strings.Replace(desc, "Authorized to ", "", 1)
	desc = strings.Replace(desc, "Grants access to ", "", 1)
	desc = strings.Replace(desc, "Allows users to ", "", 1)
	desc = strings.Replace(desc, "Access and administer ", "Administer ", 1)
	desc = strings.Replace(desc, " to all ", " to ", 1)
	desc = strings.Replace(desc, "administer all ", "administer ", 1)
	desc = strings.Replace(desc, " to get and list ", " to ", 1)
	desc = strings.Replace(desc, "Admin(super user)", "Admin ", 1)
	desc = strings.Replace(desc, "the Kubernetes Engine service account in the host 	", "GKE SA ", 1)
	desc = strings.Replace(desc, "standard (non-administrator) ", "standard ", 1)
	desc = strings.Replace(desc, "(applicable for GCP Customer Care and Maps support)", "", 1)
	if strings.HasPrefix(desc, "Can ") {
		desc = strings.Replace(desc, "Can ", "", 1)
	}
	if strings.HasPrefix(desc, "Allows ") {
		desc = strings.Replace(desc, "Allows ", "", 1)
	}
	desc = strings.ReplaceAll(desc, "  ", " ")
	desc = strings.TrimSuffix(desc, ".")
	desc = strings.TrimSpace(desc)

	klog.Infof("short string: %s (%s) from %s (%s)", id, desc, gr.Name, gr.Description)

	if desc == "" {
		return id
	}
	return fmt.Sprintf("%s (%s)", id, strings.TrimSpace(desc))
}

func gcpRoles(project string) (map[string]gcpRole, error) {
	roles := map[string]gcpRole{}

	// global roles vs local
	cmds := [][]string{
		{"iam", "roles", "list"},
		{"iam", "roles", "list", fmt.Sprintf("--project=%s", project)},
	}

	for _, args := range cmds {
		cmd := exec.Command("gcloud", args...)
		klog.Infof("executing %s", cmd)
		stdout, err := cmd.Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
			}
			return nil, fmt.Errorf("%s: %w", cmd, err)
		}
		// klog.Infof("roles output: %s", stdout)

		dec := yaml.NewDecoder(bytes.NewReader(stdout))
		for {
			var doc gcpRole
			if err := dec.Decode(&doc); err != nil {
				if !errors.Is(err, io.EOF) {
					return nil, fmt.Errorf("decode: %v", err)
				}
				break
			}
			roles[doc.Name] = doc
			// klog.Infof("gcp role: %+v", doc)
		}
	}
	return roles, nil
}

// GoogleCloudProjectIAM uses gcloud to generate a list of GCP members.
type GoogleCloudProjectIAM struct{}

func (p *GoogleCloudProjectIAM) Description() ProcessorDescription {
	hiddenRoles := []string{}
	for k := range hideRoles {
		hiddenRoles = append(hiddenRoles, k)
	}

	return ProcessorDescription{
		Kind: "gcp",
		Name: "Google Cloud Project IAM Policies",
		Steps: []string{
			"Execute 'yacls --kind={{.Kind}} --project={{.Project}}'",
		},
		NoInputRequired: true,
		Filter:          map[string][]string{"role": hiddenRoles},
	}
}

func shortName(gid gcpIdentity, orgs []string) string {
	shortName := gid.Email

	if len(orgs) == 1 {
		shortName = strings.ReplaceAll(shortName, fmt.Sprintf("@%s", orgs[0]), "")
	}
	return shortName
}

func (p *GoogleCloudProjectIAM) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	a := &Artifact{Metadata: src}

	project := c.Project
	cmd := exec.Command("gcloud", "projects", "get-ancestors-iam-policy", project)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
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

	users := map[string]*User{}
	memberships := map[string][]string{}
	roles, err := gcpRoles(c.Project)
	if err != nil {
		return nil, fmt.Errorf("gcp roles: %v", err)
	}
	klog.V(1).Infof("found roles: %+v", roles)

	sas, err := serviceAccountList(c.Project)
	if err != nil {
		return nil, fmt.Errorf("gcp sa: %v", err)
	}
	klog.V(1).Infof("service account metadata: %+v", sas)

	orgs, err := organizationsList()
	if err != nil {
		return nil, fmt.Errorf("gcp orgs: %v", err)
	}

	pnum, err := projectNumber(project)
	if err != nil {
		return nil, fmt.Errorf("project number: %v", err)
	}

	klog.V(1).Infof("project number: %s - orgs: %v", pnum, orgs)

	pbn, err := projectsByNumber()
	if err != nil {
		return nil, fmt.Errorf("projects by number: %v", err)
	}
	klog.V(1).Infof("projects by number: %+v", pbn)

	// CONFUSION ALERT!@$!@$!@
	// Google Groups have a single role (owner, member, admin) - we refer to those as "role"
	// GCP has multiple roles (role/viewer, etc) - we refer to those as "permissions" to avoid confusion
	groups := map[string]*Group{}

	identityProject := c.GCPIdentityProject
	if identityProject == "" {
		identityProject = c.Project
	}

	// YAML is parsed, lets figure out the users & roles
	for _, d := range docs {
		if a.Metadata.ID == "" {
			a.Metadata.ID = d.ID
			a.Metadata.Name = fmt.Sprintf("Google Cloud IAM Policy for %s", d.ID)
		}

		// Bindings are a relationship of roles to members
		for _, binding := range d.Policy.Bindings {
			role, found := roles[binding.Role]
			if !found {
				role = gcpRole{
					Name:        binding.Role,
					Description: "Custom",
				}
				klog.Infof("%q not in role list", binding.Role)
			}

			log.Printf("binding: %+v", binding)
			// bindMembers may be individuals or groups
			for _, bindMember := range binding.Members {
				if hideRoles[binding.Role] {
					klog.Infof("filtered role for %s: %s", bindMember, binding.Role)
					continue
				}

				id := parseGCPIdentity(bindMember)
				key := shortName(id, orgs)

				switch id.Kind {
				case "domain":
					continue
				case "serviceAccount", "deleted:serviceAccount":
					if users[bindMember] == nil {
						sa := sas[id.Email]
						klog.Infof("service account info for %q: %+v", id.Email, sa)
						u := &User{
							Name:    sa.DisplayName,
							Deleted: id.Deleted,
						}

						// Attempt to distinguish what GCP project this SA came from
						u.Project = pbn[id.Username]
						if pbn[fmt.Sprintf("service-%s", id.Username)] != "" {
							u.Project = pbn[fmt.Sprintf("service-%s", id.Username)]
						}

						users[bindMember] = u
					}
					if id.Deleted {
						users[bindMember].Deleted = true
					}
					users[bindMember].Roles = append(users[bindMember].Roles, role.String())
					memberships[key] = append(memberships[bindMember], "DIRECT")
				case "user":
					if users[bindMember] == nil {
						u := &User{}
						users[bindMember] = u
					}
					users[bindMember].Roles = append(users[bindMember].Roles, role.String())
					memberships[key] = append(memberships[bindMember], "DIRECT")
				case "group":
					expanded, err := expandGCPMembers(id.Email, identityProject, c.GCPMemberCache)
					if err != nil {
						return nil, fmt.Errorf("expand members %s: %w", bindMember, err)
					}

					// klog.Infof("m: %s -- role %s / members %s / expanded: %s", id.Email, binding.Role, binding.Members, expanded)
					_, grp, _ := strings.Cut(bindMember, ":")
					if groups[grp] == nil {
						g := &Group{}
						groups[grp] = g
					}
					groups[grp].Roles = append(groups[grp].Roles, role.String())
					for _, gm := range expanded {
						key = shortName(parseGCPIdentity(gm.Member.ID), orgs)
						memberships[key] = append(memberships[key], shortName(id, orgs))
					}
				default:
					return a, fmt.Errorf("unknown binding type %v: %v", id.Kind, bindMember)

				}

			}
		}
	}

	a.Permissions.Groups = map[string]Group{}

	for identity, g := range groups {
		key := shortName(parseGCPIdentity(identity), orgs)
		a.Permissions.Groups[key] = *g
	}

	a.Memberships = map[string]string{}
	for k, v := range memberships {
		if internalServiceAccount(k, pnum) {
			continue
		}
		sort.Strings(v)
		ms := slices.Compact(v)
		a.Memberships[k] = strings.Join(ms, ",")
	}

	a.Permissions.ServiceAccounts = map[string]User{}
	a.Permissions.Users = map[string]User{}

	for identity, u := range users {
		gid := parseGCPIdentity(identity)
		key := shortName(gid, orgs)
		if gid.Kind == "serviceAccount" {
			if internalServiceAccount(key, pnum) {
				continue
			}
			key = strings.ReplaceAll(key, ".gserviceaccount.com", "")
			if gid.Username == u.Name {
				u.Name = ""
			}
			a.Permissions.ServiceAccounts[key] = *u
			continue
		}

		a.Permissions.Users[key] = *u
	}

	return a, nil
}

// GCP by default hides internal service accounts, this lets you find them
func internalServiceAccount(identity string, pnum string) bool {
	gid := parseGCPIdentity(identity)
	internal := false
	if strings.HasPrefix(gid.Email, "service-") && strings.HasSuffix(gid.Email, ".iam.gserviceaccount.com") {
		internal = true
	}
	if strings.HasPrefix(gid.Email, fmt.Sprintf("%s@", pnum)) && strings.HasSuffix(gid.Email, "cloudservices.gserviceaccount.com") {
		internal = true
	}
	return internal
}
