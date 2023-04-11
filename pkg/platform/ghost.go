package platform

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"k8s.io/klog/v2"
)

var ghostUserRe = regexp.MustCompile(`/staff/([\w-]+)`)

// GhostStaff parses the HTML output of the Ghost Staff page.
type GhostStaff struct{}

func (p *GhostStaff) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "ghost",
		Name: "Ghost Blog Permissions",
		Steps: []string{
			"Open the corporate Ghost blog",
			"Click 'Settings'",
			"Click 'Staff'",
			"Zoom out so that all users are visible on one screen",
			"Save this page (Complete)",
			"Collect resulting .html file for analysis (the other files are not necessary)",
			"Execute 'yacls --kind={{.Kind}} --input={{.Path}}'",
		},
	}
}

func (p *GhostStaff) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	a := &Artifact{Metadata: src}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(src.content))
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}

	// Check each link to see if it seems to be a staff link
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		matches := ghostUserRe.FindStringSubmatch(s.AttrOr("href", ""))
		if len(matches) < 2 {
			return
		}
		klog.Infof("found matching link: %s", s.AttrOr("href", ""))

		username := matches[1]
		name := s.Find("h3").Text()
		role := s.Find("span.gh-badge").Text()
		a.Users = append(a.Users, User{Account: username, Name: name, Role: role})
	})

	return a, nil
}
