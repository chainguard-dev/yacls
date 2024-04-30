package platform

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var SourceDateFormat = "2006-01-02"

type Artifact struct {
	Metadata  *Source
	UserCount int    `yaml:"users_total,omitempty"`
	Users     []User `yaml:"users,omitempty"`

	Ingress []FirewallRuleMeta `yaml:"ingress,omitempty"`
	Egress  []FirewallRuleMeta `yaml:"egress,omitempty"`

	BotCount int    `yaml:"bots_total,omitempty"`
	Bots     []User `yaml:"bots,omitempty"`

	ServiceAccountCount int    `yaml:"service_accounts_total,omitempty"`
	ServiceAccounts     []User `yaml:"service_accounts,omitempty"`

	GroupCount  int                 `yaml:"groups_total,omitempty"`
	Groups      []Group             `yaml:"groups,omitempty"`
	OrgCount    int                 `yaml:"orgs_total,omitempty"`
	Orgs        []Group             `yaml:"orgs,omitempty"`
	RoleCount   int                 `yaml:"roles_total,omitempty"`
	Roles       map[string][]string `yaml:"roles,omitempty"`
	Permissions Permissions         `yaml:"permissions,omitempty"`

	Memberships map[string]string `yaml:"membership,omitempty"`
}

type Permissions struct {
	UserCount           int              `yaml:"users_total,omitempty"`
	Users               map[string]User  `yaml:"users,omitempty"`
	ServiceAccountCount int              `yaml:"service_accounts_total,omitempty"`
	ServiceAccounts     map[string]User  `yaml:"service_accounts,omitempty"`
	GroupCount          int              `yaml:"groups_total,omitempty"`
	Groups              map[string]Group `yaml:"groups,omitempty"`
}

type FirewallRuleMeta struct {
	Name        string
	Description string `yaml:"description,omitempty"`
	Logging     bool   `yaml:"logging,omitempty"`
	Priority    int    `yaml:"priority,omitempty"`
	Rule        FirewallRule
}

// FirewallRule
type FirewallRule struct {
	Allow        string `yaml:"allow,omitempty"`
	Deny         string `yaml:"deny,omitempty"`
	Network      string `yaml:"net,omitempty"`
	Sources      string `yaml:"sources,omitempty"`
	Destinations string `yaml:"destinations,omitempty"`
	SourceTags   string `yaml:"source_tags,omitempty"`
	TargetTags   string `yaml:"target_tags,omitempty"`
}

type User struct {
	Account           string       `yaml:",omitempty"`
	Name              string       `yaml:",omitempty"`
	Email             string       `yaml:",omitempty"`
	Role              string       `yaml:",omitempty"`
	Roles             []string     `yaml:"roles,omitempty"`
	Permissions       []string     `yaml:",omitempty"`
	Project           string       `yaml:"project,omitempty"`
	Status            string       `yaml:",omitempty"`
	Groups            []Membership `yaml:",omitempty"`
	Org               string       `yaml:",omitempty"`
	Deleted           bool         `yaml:",omitempty"`
	TwoFactorDisabled bool         `yaml:"two_factor_disabled,omitempty"`
	SSO               string       `yaml:"sso,omitempty"`
}

type Group struct {
	Name        string   `yaml:",omitempty"`
	Description string   `yaml:",omitempty"`
	Permissions []string `yaml:"permissions,omitempty"`
	Roles       []string `yaml:"roles,omitempty"`
	Members     []string `yaml:"members,omitempty"`
}

type Membership struct {
	Name        string   `yaml:",omitempty"`
	Description string   `yaml:",omitempty"`
	Role        string   `yaml:",omitempty"`
	Permissions []string `yaml:"permissions,omitempty"`
}

type Source struct {
	Kind        string
	Name        string
	ID          string    `yaml:",omitempty"`
	SourceDate  string    `yaml:"source_date,omitempty"`
	GeneratedAt time.Time `yaml:"generated_at"`
	GeneratedBy string    `yaml:"generated_by"`
	Process     []string

	content []byte
}

// NewSourceFromConfig begins processing a source file, returning a source struct.
func NewSourceFromConfig(c Config, p Processor) (*Source, error) {
	var content []byte
	var err error

	if c.Reader != nil {
		content, err = io.ReadAll(c.Reader)
		if err != nil {
			return nil, fmt.Errorf("readall: %w", err)
		}
	}

	mtime := time.Now()
	if c.Path != "" {
		fi, err := os.Stat(c.Path)
		if err != nil {
			return nil, fmt.Errorf("stat: %w", err)
		}

		mtime = fi.ModTime()
	}

	cu, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("user: %w", err)
	}

	desc := p.Description()
	return &Source{
		GeneratedAt: time.Now(),
		GeneratedBy: cu.Username,
		SourceDate:  mtime.Format(SourceDateFormat),
		content:     content,
		Kind:        desc.Kind,
		Name:        desc.Name,
		Process:     renderSteps(desc.Steps, c),
	}, nil
}

// FinalizeArtifact does some final manipulation on an artifact for consistency.
func FinalizeArtifact(a *Artifact) {
	// Make the output more deterministic
	sort.Slice(a.Users, func(i, j int) bool {
		return a.Users[i].Account < a.Users[j].Account
	})
	sort.Slice(a.Bots, func(i, j int) bool {
		return a.Bots[i].Account < a.Bots[j].Account
	})
	sort.Slice(a.Orgs, func(i, j int) bool {
		return a.Orgs[i].Name < a.Orgs[j].Name
	})

	sort.Slice(a.Ingress, func(i, j int) bool {
		if a.Ingress[i].Priority != a.Ingress[j].Priority {
			return a.Ingress[i].Priority < a.Ingress[j].Priority
		}
		return a.Ingress[i].Name < a.Ingress[j].Name
	})
	sort.Slice(a.Egress, func(i, j int) bool {
		if a.Egress[i].Priority != a.Egress[j].Priority {
			return a.Egress[i].Priority < a.Egress[j].Priority
		}
		return a.Egress[i].Name < a.Egress[j].Name
	})

	//	a.ByGroup = map[string][]Membership{}
	a.Roles = map[string][]string{}
	//	a.Permissions = map[string][]string{}

	allUsers := []User{}
	allUsers = append(allUsers, a.Users...)
	allUsers = append(allUsers, a.Bots...)
	//	hasPermission := map[string]map[string]bool{}

	for _, u := range allUsers {
		if u.Role != "" {
			a.Roles[u.Role] = append(a.Roles[u.Role], u.Account)
		}

		/*
			perms := u.Permissions
			for _, p := range perms {
				if a.Permissions[p] == nil {
					a.Permissions[p] = []string{}
					hasPermission[p] = map[string]bool{}
				}
				a.Permissions[p] = append(a.Permissions[p], u.Account)
				hasPermission[p][u.Account] = true
			}
		*/
	}

	/*
		// Deal with inherited permissions
		for _, g := range a.Groups {
			for _, m := range g.Members {
				for _, p := range g.Permissions {
					if hasPermission[p] == nil {
						hasPermission[p] = map[string]bool{}
					}

					if !hasPermission[p][m] {
						a.Permissions[p] = append(a.Permissions[p], m)
						hasPermission[p][m] = true
					}
				}
			}
		}
	*/

	/*
		for i := range a.Permissions {
			sort.Strings(a.Permissions[i])
		}
	*/

	a.UserCount = len(a.Users)
	a.BotCount = len(a.Bots)
	// a.PermissionCount = len(a.Permissions)
	a.RoleCount = len(a.Roles)
	a.GroupCount = len(a.Groups)
	a.OrgCount = len(a.Orgs)
}

// updates {{.Path}} or {{.Project}} in a list of steps.
func renderSteps(steps []string, c Config) []string {
	// Dummy output
	if c.Path == "" {
		c.Path = "<path>"
	}
	if c.Project == "" {
		c.Project = "<project>"
	}

	out := []string{}
	for _, r := range steps {
		r := r
		t, err := template.New("step").Parse(r)
		if err != nil {
			panic(fmt.Sprintf("unable to parse step %q: %v", r, err))
		}

		bs := bytes.NewBufferString("")
		err = t.Execute(bs, c)
		if err != nil {
			panic(fmt.Sprintf("unable to parse step %q: %v", r, err))
		}

		out = append(out, bs.String())
	}
	return out
}

type ProcessorDescription struct {
	Kind             string
	Name             string
	Steps            []string
	OptionalFields   []string
	MatchingFilename *regexp.Regexp
	Filter           map[string][]string `yaml:"filter"`

	NoInputRequired bool
}

type Config struct {
	Path               string
	Reader             io.Reader
	Project            string
	Kind               string
	GCPIdentityProject string

	GCPMemberCache GCPMemberCache
}

type Processor interface {
	Description() ProcessorDescription
	Process(c Config) (*Artifact, error)
}

func New(kind string) (Processor, error) {
	for _, p := range Available() {
		if kind == p.Description().Kind {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown kind: %q", kind)
}

func SuggestKind(path string) (string, error) {
	base := filepath.Base(path)
	for _, p := range Available() {
		if p.Description().MatchingFilename != nil && p.Description().MatchingFilename.MatchString(base) {
			return p.Description().Kind, nil
		}
		if strings.HasPrefix(base, p.Description().Kind) {
			return p.Description().Kind, nil
		}
	}
	return "", fmt.Errorf("unable to find kind for %q", path)
}

func Available() []Processor {
	// Alphabetical
	return []Processor{
		&Auth0Members{},
		&DockerHubMembers{},
		&GhostStaff{},
		&GithubOrgMembers{},
		&GoogleCloudProjectIAM{},
		&GoogleCloudProjectFirewall{},
		&GoogleWorkspaceUserAudit{},
		&GoogleWorkspaceUsers{},
		&KolideUsers{},
		&OnePasswordTeam{},
		&pulumiPeople{},
		&SecureframePersonnel{},
		&SlackMembers{},
		&VercelMembers{},
		&WebflowMembers{},
	}
}

func AvailableKinds() []string {
	kinds := []string{}
	for _, p := range Available() {
		kinds = append(kinds, p.Description().Kind)
	}
	sort.Strings(kinds)
	return kinds
}
