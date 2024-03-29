package platform

import (
	"fmt"
	"strings"

	"github.com/gocarina/gocsv"
)

// SecureframePersonnel parses the CSV file generated by the Secureframe Personnel page.
type SecureframePersonnel struct{}

func (p *SecureframePersonnel) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "secureframe",
		Name: "Secureframe Personnel",
		Steps: []string{
			"Open https://app.secureframe.com/personnel",
			"Deselect any active filters",
			"Click Export...",
			"Select 'Direct Download'",
			"Download resulting CSV file for analysis",
			"Execute 'yacls --kind={{.Kind}} --input={{.Path}}'",
		},
	}
}

type secureframePersonnelRecord struct {
	Email string `csv:"Name (email)"`
	Role  string `csv:"Access role"`
}

func (p *SecureframePersonnel) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	a := &Artifact{Metadata: src}

	records := []secureframePersonnelRecord{}
	if err := gocsv.UnmarshalBytes(src.content, &records); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	for _, r := range records {
		if r.Role == "" {
			continue
		}

		id, _, _ := strings.Cut(r.Email, "@")
		u := User{
			Account: id,
			Role:    r.Role,
		}

		a.Users = append(a.Users, u)
	}

	return a, nil
}
