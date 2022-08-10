package axs

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/gocarina/gocsv"
	"k8s.io/klog/v2"
)

var (
	googleWorkspaceAuditSteps = []string{
		"Open https://admin.google.com/ac/reporting/report/user/accounts",
		"Click Download icon",
		"Select All Columns",
		"Click CSV",
		"Execute 'axsdump --google-workspace-audit-csv=<path>'",
	}

	googleAuditDateRegexp = regexp.MustCompile(` \[(\d{4}-\d{2}-\d{2}) GMT\]`)
)

type googleWorkspaceAuditRecord struct {
	User        string `csv:"User"`
	Status      string `csv:"User account status"`
	AdminStatus string `csv:"Admin status"`
	Name        string `csv:"Admin-defined name"`
}

// GoogleWorkspaceUserAudit parses the CSV file generated by the Google User Audit page.
func GoogleWorkspaceAudit(path string) (*Artifact, error) {
	src, err := NewSource(path)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	src.Kind = "google_workspace_audit"
	src.Name = "Google Workspace User Audit"
	src.Process = googleWorkspaceAuditSteps
	a := &Artifact{Metadata: src}

	neutered, date := extractDateFromHeaders(src.content)
	src.SourceDate = date

	records := []googleWorkspaceAuditRecord{}
	if err := gocsv.UnmarshalString(neutered, &records); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	for _, r := range records {
		username, _, _ := strings.Cut(r.User, "@")
		u := User{
			Account: username,
			// The most important thing about this audit is permissions
			// 	Name:    r.Name,
		}

		if r.AdminStatus != "None" {
			u.Role = r.AdminStatus
		}

		if r.Status != "Active" {
			u.Status = r.Status
		}
		a.Users = append(a.Users, u)
	}

	return a, nil
}

func extractDateFromHeaders(bs []byte) (string, string) {
	s := bufio.NewScanner(bytes.NewReader(bs))
	s.Split(bufio.ScanLines)
	lines := []string{}
	date := ""

	for s.Scan() {
		line := s.Text()
		if len(lines) == 0 {
			matches := googleAuditDateRegexp.FindStringSubmatch(line)
			if len(matches) > 1 {
				klog.Infof("found string: %s", matches[1])
				date = matches[1]
			}
			line = googleAuditDateRegexp.ReplaceAllString(line, "")
		}

		klog.Infof("line: %s", line)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), date
}
