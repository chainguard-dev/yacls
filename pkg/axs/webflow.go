package axs

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var WebflowSteps = []string{
	"Open https://webflow.com/dashboard/sites/<site>/members",
	"Save this page (Complete)",
	"Collect resulting .html file for analysis (the other files are not necessary)",
	"Execute 'axsdump --webflow-members-html=<path>'",
}

var webflowUserRe = regexp.MustCompile(`(.*?) \((.*?@.*?)\)`)

// WebflowMembers parses the HTML output of the Webflow Member page.
func WebflowMembers(path string) (*Artifact, error) {
	src, err := NewSource(path)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	src.Kind = "webflow_members"
	src.Name = "Webflow Site Permissions"
	src.Process = renderSteps(WebflowSteps, path)
	a := &Artifact{Metadata: src}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(src.content))
	if err != nil {
		return nil, fmt.Errorf("document: %w", err)
	}

	// Find the members
	doc.Find("tr.member").Each(func(i int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Find("div.ng-binding").Text())
		if raw == "" {
			return
		}

		matches := webflowUserRe.FindStringSubmatch(raw)
		if len(matches) < 2 {
			return
		}

		name := matches[1]
		account := matches[2]
		role := strings.TrimSpace(s.Find("span.ng-binding").Text())

		a.Users = append(a.Users, User{Account: account, Name: name, Role: role})
	})

	return a, nil
}
