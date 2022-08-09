package axs

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

var ghostSteps = []string{
	"Open the corporate Ghost blog",
	"Click 'Settings'",
	"Click 'Staff'",
	"Save this page (Complete)",
	"Execute 'axsdump --ghost-staff-html=<path.html>'",
}

var ghostUserRe = regexp.MustCompile(`/staff/(\w+)`)

// GhostStaffs parses the HTML output of the Ghost Staff page.
func GhostStaff(path string) (*Artifact, error) {
	src, err := NewSource(path)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	src.Kind = "ghost_staff"
	src.Name = "Ghost Blog Permissions"
	src.Process = ghostSteps
	a := &Artifact{Metadata: src}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(src.content))
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}

	// Find the review items
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		matches := ghostUserRe.FindStringSubmatch(s.AttrOr("href", ""))
		if len(matches) < 2 {
			return
		}

		username := matches[1]

		name := s.Find("h3").Text()
		role := s.Find("span.gh-badge").Text()
		fmt.Printf("role=%s", role)
		a.Users = append(a.Users, User{Account: username, Name: name, Permissions: []string{role}})
	})

	return a, nil
}
