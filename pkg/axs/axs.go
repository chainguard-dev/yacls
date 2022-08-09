package axs

import "time"

var SourceDateFormat = "2006-01-02"

type Artifact struct {
	Kind         string
	Name         string
	SourceDate   string    `yaml:"source_date,omitempty"`
	GeneratedAt  time.Time `yaml:"generated_at"`
	GeneratedBy  string    `yaml:"generated_by"`
	Process      []string
	ByPermission map[string][]string `yaml:",omitempty"`
	Users        []User
}

type User struct {
	Account     string
	Name        string
	Permissions []string `yaml:",omitempty"`
	Status      string   `yaml:",omitempty"`
}
