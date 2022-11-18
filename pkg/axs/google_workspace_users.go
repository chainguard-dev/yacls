package axs

import (
	"fmt"
	"strings"

	"github.com/gocarina/gocsv"
)

var GoogleWorkspaceUsersSteps = []string{
	"Open https://admin.google.com/ac/users",
	"Click Download users",
	"Select 'All user info Columns'",
	"Select 'Comma-separated values (.csv)'",
	"Download resulting CSV file for analysis",
	"Execute 'acls-in-yaml --google-workspace-users-csv=<path>'",
}

type googleWorkspaceUserRecord struct {
	EmailAddress      string `csv:"Email Address [Required]"`
	Status            string `csv:"Status [READ ONLY]"`
	FirstName         string `csv:"First Name [Required]"`
	LastName          string `csv:"Last Name [Required]"`
	LastSignIn        string `csv:"Last Sign In [READ ONLY]"`
	TwoFactorEnforced string `csv:"2sv Enforced [READ ONLY]"`
}

// GoogleWorkspaceUsers parses the CSV file generated by the users page.
func GoogleWorkspaceUsers(path string) (*Artifact, error) {
	src, err := NewSource(path)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	src.Kind = "google_workspace_users"
	src.Name = "Google Workspace User List"
	src.Process = renderSteps(GoogleWorkspaceUsersSteps, path)
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
