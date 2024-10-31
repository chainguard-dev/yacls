package platform

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// cloudflareMembers parses the HTML output of the cloudflare Members page.
type cloudflareMembers struct{}

func (p *cloudflareMembers) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "cloudflare",
		Name: "cloudflare Site Permissions",
		Steps: []string{
			"Open https://cloudflare.com/",
			"Select your company/team",
			"Click 'Settings'",
			"Click 'Members'",
			"Save this page (Complete)",
			"Collect resulting .html file for analysis (the other files are not necessary)",
			"Execute 'yacls --kind={{.Kind}} --input={{.Path}}'",
		},
		MatchingFilename: regexp.MustCompile(`.*Cloudflare.*.html$`),
	}
}

func (p *cloudflareMembers) Process(c Config) (*Artifact, error) {
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

	// Find the Members
	doc.Find("div[role=row]").Each(func(i int, row *goquery.Selection) {
		u := User{}
		// Holds e-mail address or role
		row.Find("div.c_sx").Each(func(j int, div *goquery.Selection) {
			val := div.Text()
			if strings.Contains(val, "@") {
				u.Account = strings.TrimSpace(val)
				return
			}
			u.Role = strings.TrimSpace(val)
			u.Role, _, _ = strings.Cut(u.Role, " - ")
		})

		// Holds account status or 2FA
		row.Find("span.c_lf").Each(func(j int, div *goquery.Selection) {
			val := div.Text()
			// Account Status
			if j == 0 && val != "Active" {
				u.Deleted = true
			}
			// 2FA Status
			if j == 1 && val != "Enabled" {
				u.TwoFactorDisabled = true
			}
		})
		a.Users = append(a.Users, u)
	})

	return a, nil
}
