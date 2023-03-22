package platform

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/user"
	"sort"
	"time"
)

var SourceDateFormat = "2006-01-02"

type Artifact struct {
	Metadata        *Source
	UserCount       int `yaml:"user_count"`
	Users           []User
	BotCount        int                 `yaml:"bot_count,omitempty"`
	Bots            []User              `yaml:",omitempty"`
	GroupCount      int                 `yaml:"group_count,omitempty"`
	Groups          []Group             `yaml:"groups,omitempty"`
	OrgCount        int                 `yaml:"org_count,omitempty"`
	Orgs            []Group             `yaml:"orgs,omitempty"`
	RoleCount       int                 `yaml:"role_count,omitempty"`
	Roles           map[string][]string `yaml:"roles,omitempty"`
	PermissionCount int                 `yaml:"permission_count,omitempty"`
	Permissions     map[string][]string `yaml:"permissions,omitempty"`
}

type User struct {
	Account           string
	Name              string       `yaml:",omitempty"`
	Role              string       `yaml:",omitempty"`
	Permissions       []string     `yaml:",omitempty"`
	Status            string       `yaml:",omitempty"`
	Groups            []Membership `yaml:",omitempty"`
	Org               string       `yaml:",omitempty"`
	TwoFactorDisabled bool         `yaml:"two_factor_disabled,omitempty"`
	SSO               string       `yaml:"sso,omitempty"`
}

type Group struct {
	Name        string   `yaml:",omitempty"`
	Description string   `yaml:",omitempty"`
	Permissions []string `yaml:"permissions,omitempty"`
	Members     []string
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

	//	a.ByGroup = map[string][]Membership{}
	a.Roles = map[string][]string{}
	a.Permissions = map[string][]string{}

	allUsers := []User{}
	allUsers = append(allUsers, a.Users...)
	allUsers = append(allUsers, a.Bots...)
	hasPermission := map[string]map[string]bool{}

	for _, u := range allUsers {
		if u.Role != "" {
			a.Roles[u.Role] = append(a.Roles[u.Role], u.Account)
		}

		perms := u.Permissions
		for _, p := range perms {
			if a.Permissions[p] == nil {
				a.Permissions[p] = []string{}
				hasPermission[p] = map[string]bool{}
			}
			a.Permissions[p] = append(a.Permissions[p], u.Account)
			hasPermission[p][u.Account] = true
		}
	}

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

	for i := range a.Permissions {
		sort.Strings(a.Permissions[i])
	}

	a.UserCount = len(a.Users)
	a.BotCount = len(a.Bots)
	a.PermissionCount = len(a.Permissions)
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
	Kind           string
	Name           string
	Steps          []string
	OptionalFields []string
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

func Available() []Processor {
	// Alphabetical
	return []Processor{
		&Auth0Members{},
		&GhostStaff{},
		&GithubOrgMembers{},
		&GoogleCloudProjectIAM{},
		&GoogleWorkspaceUserAudit{},
		&GoogleWorkspaceUsers{},
		&KolideUsers{},
		&OnePasswordTeam{},
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
