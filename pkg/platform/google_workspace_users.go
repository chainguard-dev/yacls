package platform

import (
	"fmt"
	"strings"

	"github.com/gocarina/gocsv"
)

// GoogleWorkspaceUsers parses the CSV file generated by the users page.
type GoogleWorkspaceUsers struct{}

func (p *GoogleWorkspaceUsers) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "google-workspace-users",
		Name: "Google Workspace Users",
		Steps: []string{
			"Open https://admin.google.com/ac/users",
			"Click Download users",
			"Select 'All user info Columns'",
			"Select 'Comma-separated values (.csv)'",
			"Download resulting CSV file for analysis",
			"Execute 'acls-in-yaml --google-workspace-users-csv={{.Path}}'",
		},
	}
}

type googleWorkspaceUserRecord struct {
	EmailAddress      string `csv:"Email Address [Required]"`
	Status            string `csv:"Status [READ ONLY]"`
	FirstName         string `csv:"First Name [Required]"`
	LastName          string `csv:"Last Name [Required]"`
	LastSignIn        string `csv:"Last Sign In [READ ONLY]"`
	TwoFactorEnforced string `csv:"2sv Enforced [READ ONLY]"`
}

func (p *GoogleWorkspaceUsers) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	a := &Artifact{Metadata: src}
	records := []googleWorkspaceUserRecord{}
	if err := gocsv.UnmarshalBytes(src.content, &records); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	for _, r := range records {
		username, _, _ := strings.Cut(r.EmailAddress, "@")
		u := User{
			Account: username,
			Name:    strings.TrimSpace(r.FirstName) + " " + strings.TrimSpace(r.LastName),
		}

		if r.Status != "Active" {
			u.Status = r.Status
		}
		a.Users = append(a.Users, u)
	}

	return a, nil
}
