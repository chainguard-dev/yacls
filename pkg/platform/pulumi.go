package platform

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// pulumiPeople parses the HTML output of the pulumi People page.
type pulumiPeople struct{}

func (p *pulumiPeople) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "pulumi",
		Name: "pulumi Site Permissions",
		Steps: []string{
			"Open https://pulumi.com/",
			"Select your company/team",
			"Click 'Settings'",
			"Click 'People'",
			"Save this page (Complete)",
			"Collect resulting .html file for analysis (the other files are not necessary)",
			"Execute 'yacls --kind={{.Kind}} --input={{.Path}}'",
		},
		MatchingFilename: regexp.MustCompile(`Pulumi.*.html$`),
	}
}

func (p *pulumiPeople) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	a := &Artifact{Metadata: src}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(src.content))
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}

	// Find the People
	doc.Find(".cdk-row").Each(func(i int, s *goquery.Selection) {
		email := strings.TrimSpace(s.Find("a.login").Text())
		name := strings.TrimSpace(s.Find("p.name").Text())
		role := strings.TrimSpace(s.Find("span.ng-star-inserted").Text())
		status := strings.TrimSpace(s.Find("div.invite-status-container").Text())
		// bad workaround for "membermembermember" drop-down
		if strings.HasPrefix(role, "member") {
			role = "member"
		}
		a.Users = append(a.Users, User{Account: email, Name: name, Role: role, Status: status})
	})

	return a, nil
}
