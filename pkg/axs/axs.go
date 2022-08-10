package axs

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"time"
)

var SourceDateFormat = "2006-01-02"

type Artifact struct {
	Metadata  *Source
	UserCount int `yaml:"user_count"`
	Users     []User
	Bots      []User                  `yaml:",omitempty"`
	ByRole    map[string][]string     `yaml:"by-role,omitempty"`
	ByGroup   map[string][]Membership `yaml:"by-group,omitempty"`
}

type User struct {
	Account string
	Name    string       `yaml:",omitempty"`
	Role    string       `yaml:",omitempty"`
	Roles   []string     `yaml:",omitempty"` // for systems that support multiple roles, like GCP
	Status  string       `yaml:",omitempty"`
	Groups  []Membership `yaml:",omitempty"`
}

type Membership struct {
	Name        string   `yaml:",omitempty"`
	Description string   `yaml:",omitempty"`
	Role        string   `yaml:",omitempty"`
	Roles       []string `yaml:",omitempty"`
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

// FinalizeArtifact does some final manipulation on an artifact for consistency.
func FinalizeArtifact(a *Artifact) {
	// Make the output more deterministic
	sort.Slice(a.Users, func(i, j int) bool {
		return a.Users[i].Account < a.Users[j].Account
	})
	sort.Slice(a.Bots, func(i, j int) bool {
		return a.Bots[i].Account < a.Bots[j].Account
	})

	a.UserCount = len(a.Users)
	a.ByGroup = map[string][]Membership{}
	a.ByRole = map[string][]string{}

	allUsers := []User{}
	allUsers = append(allUsers, a.Users...)
	allUsers = append(allUsers, a.Bots...)

	for _, u := range allUsers {
		for _, g := range u.Groups {
			if a.ByGroup[g.Name] == nil {
				a.ByGroup[g.Name] = []Membership{}
			}
			a.ByGroup[g.Name] = append(a.ByGroup[g.Name], Membership{Name: u.Account, Role: g.Role, Roles: g.Roles})
		}

		roles := u.Roles
		if len(roles) == 0 && u.Role != "" {
			roles = []string{u.Role}
		}

		for _, r := range roles {
			if a.ByRole[r] == nil {
				a.ByRole[r] = []string{}
			}
			a.ByRole[r] = append(a.ByRole[r], u.Account)
		}
	}
}
