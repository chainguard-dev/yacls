package axs

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"
	"time"
)

var SourceDateFormat = "2006-01-02"

type Artifact struct {
	Metadata *Source
	Users    []User
	Bots     []User              `yaml:",omitempty"`
	ByRole   map[string][]string `yaml:"by-role,omitempty"`
}

type User struct {
	Account string
	Name    string   `yaml:",omitempty"`
	Role    string   `yaml:",omitempty"`
	Roles   []string `yaml:",omitempty"` // for systems that support multiple roles, like GCP
	Status  string   `yaml:",omitempty"`
}

type Source struct {
	Kind        string
	Name        string
	ID          string
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

	roleMap := map[string][]string{}
	for i, u := range a.Users {
		if u.Role == "" {
			continue
		}
		a.Users[i].Role = strings.ToLower(u.Role)
		k := a.Users[i].Role
		if roleMap[k] == nil {
			roleMap[k] = []string{}
		}
		roleMap[k] = append(roleMap[k], u.Account)
	}

	for i, u := range a.Bots {
		if u.Role == "" {
			continue
		}
		a.Bots[i].Role = strings.ToLower(u.Role)
		k := a.Bots[i].Role
		if roleMap[k] == nil {
			roleMap[k] = []string{}
		}
		roleMap[k] = append(roleMap[k], u.Account)
	}

	a.ByRole = roleMap
}
