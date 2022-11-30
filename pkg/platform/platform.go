package platform

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
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
	RoleCount       int                 `yaml:"role_count,omitempty"`
	Roles           map[string][]string `yaml:"roles,omitempty"`
	PermissionCount int                 `yaml:"permission_count,omitempty"`
	Permissions     map[string][]string `yaml:"permissions,omitempty"`
}

type User struct {
	Account     string
	Name        string       `yaml:",omitempty"`
	Role        string       `yaml:",omitempty"`
	Permissions []string     `yaml:",omitempty"`
	Status      string       `yaml:",omitempty"`
	Groups      []Membership `yaml:",omitempty"`
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

// NewSource begins processing a source file, returning a source struct.
func NewSource(path string) (*Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	defer f.Close()

	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}

	date := fi.ModTime()

	cu, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("user: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &Source{
		GeneratedAt: time.Now(),
		GeneratedBy: cu.Username,
		SourceDate:  date.Format(SourceDateFormat),
		content:     content,
	}, nil
}

// NewSourceFromConfig begins processing a source file, returning a source struct.
func NewSourceFromConfig(c Config, p Processor) (*Source, error) {

	content, err := ioutil.ReadAll(c.Reader)
	if err != nil {
		return nil, fmt.Errorf("readall: %w", err)
	}

	var mtime time.Time
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
		Process:     renderSteps(desc.Steps, c.Path),
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
}

// updates <path> or <project> in a list of steps.
func renderSteps(steps []string, id string) []string {
	out := []string{}
	for _, r := range steps {
		rendered := r
		if strings.Contains(rendered, "<path>") {
			rendered = strings.Replace(r, "<path>", filepath.Base(id), 1)
		}

		if strings.Contains(rendered, "<project>") {
			rendered = strings.Replace(r, "<project>", id, 1)
		}

		out = append(out, rendered)
	}
	return out
}

type ProcessorDescription struct {
	Kind            string
	Name            string
	Steps           []string
	SupportsProject bool
}

type Config struct {
	Path    string
	Reader  io.Reader
	Project string
	Kind    string
}

type Processor interface {
	Description() ProcessorDescription
	Process(c Config) (*Artifact, error)
}

func New(kind string) (Processor, error) {
	switch kind {
	case "1password-team":
		return &OnePasswordTeam{}, nil
	default:
		return nil, fmt.Errorf("unknown kind: %q", kind)
	}
}

func Available() []Processor {
	return []Processor{
		&OnePasswordTeam{},
	}
}
