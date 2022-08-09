package axs

import (
	"fmt"
	"os"
	"os/user"
	"time"
)

var SourceDateFormat = "2006-01-02"

type Artifact struct {
	Metadata     *Source
	ByPermission map[string][]string `yaml:",omitempty"`
	Users        []User
	Bots         []User
}

type User struct {
	Account     string
	Name        string   `yaml:",omitempty"`
	Permissions []string `yaml:",omitempty"`
	Status      string   `yaml:",omitempty"`
}

type Source struct {
	Kind        string
	Name        string
	SourceDate  string    `yaml:"source_date,omitempty"`
	GeneratedAt time.Time `yaml:"generated_at"`
	GeneratedBy string    `yaml:"generated_by"`
	Process     []string

	content []byte
}

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
