package platform

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var VercelSteps = []string{
	"Open https://vercel.com/",
	"Select your company/team",
	"Click 'Settings'",
	"Click 'Members'",
	"Save this page (Complete)",
	"Collect resulting .html file for analysis (the other files are not necessary)",
	"Execute 'acls-in-yaml --vercel-members-html=<path>'",
}

// VercelMembers parses the HTML output of the Vercel Members page.
func VercelMembers(path string) (*Artifact, error) {
	src, err := NewSource(path)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	src.Kind = "vercel_members"
	src.Name = "Vercel Members"
	src.Process = renderSteps(VercelSteps, path)
	a := &Artifact{Metadata: src}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(src.content))
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}

	// Find the members
	doc.Find("div[data-geist-entity]").Each(func(i int, s *goquery.Selection) {
		fmt.Printf("attr=%s\n", s.AttrOr("data-testid", "unknown"))

		email := s.Find("p[type=secondary]").Text()
		roles := []string{}

		s.Find("option").Each(func(i int, opt *goquery.Selection) {
			fmt.Printf("opt=%s\n", opt.Text())
			roles = append(roles, opt.Text())
		})

		// If the user does not have access to change their permissions, it will show up here.
		if len(roles) == 0 {
			vals := []string{}

			s.Find("p").Each(func(i int, p *goquery.Selection) {
				vals = append(vals, p.Text())
			})

			title := cases.Title(language.English).String(vals[len(vals)-1])
			roles = append(roles, title)
		}

		role := roles[0]

		if len(roles) > 1 {
			// At the moment, we can't tell which option is selected
			role = strings.Join(roles, " or ")
		}

		a.Users = append(a.Users, User{Account: email, Role: role})
	})

	return a, nil
}
