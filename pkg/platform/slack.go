package platform

import (
	"fmt"
	"strings"

	"github.com/gocarina/gocsv"
)

// SlackMembers parses the HTML output of the Slack Members page.
type SlackMembers struct{}

func (p *SlackMembers) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "slack",
		Name: "Slack Members",
		Steps: []string{
			"Open Slack",
			"Click <org name>▼",
			"Select 'Settings & Administration'",
			"Select 'Manage Members'",
			"Select 'Export Member List'",
			"Download resulting CSV file for analysis",

			"Execute 'yacls --kind={{.Kind}} --input={{.Path}}'",
		},
	}
}

type slackMemberRecord struct {
	Username    string `csv:"username"`
	Email       string `csv:"email"`
	Status      string `csv:"status"`
	FullName    string `csv:"fullname"`
	DisplayName string `csv:"displayname"`
}

func (p *SlackMembers) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}

	a := &Artifact{Metadata: src}

	records := []slackMemberRecord{}
	if err := gocsv.UnmarshalBytes(src.content, &records); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	for _, r := range records {
		if r.Status == "Deactivated" {
			continue
		}

		name := strings.TrimSpace(r.FullName)
		if name == "" {
			name = strings.TrimSpace(r.DisplayName)
		}

		role := r.Status
		account := r.Email
		if role == "Bot" {
			role = ""
			account = r.Username + "!" + r.Email
		}
		if role == "Member" {
			role = ""
		}

		u := User{
			Account: account,
			Name:    name,
			Role:    role,
		}

		if r.Status == "Bot" {
			a.Bots = append(a.Bots, u)
			continue
		}
		a.Users = append(a.Users, u)
	}

	return a, nil
}
