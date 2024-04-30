package platform

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type gcloudTarget struct {
	IPProtocol string   `json:"IPProtocol"`
	Ports      []string `json:"ports"`
}

// gcloudFirewallFull is what is returned by "gcloud firewalls list"
type gcloudFirewallFull struct {
	Allowed           []gcloudTarget `json:"allowed"`
	Denied            []gcloudTarget `json:"denied"`
	CreationTimestamp string         `json:"creationTimestamp"`
	Description       string         `json:"description"`
	Direction         string         `json:"direction"`
	Disabled          bool           `json:"disabled"`
	ID                string         `json:"id"`
	Kind              string         `json:"kind"`
	LogConfig         struct {
		Enable bool `json:"enable"`
	} `json:"logConfig"`
	Name              string   `json:"name"`
	Network           string   `json:"network"`
	Priority          int      `json:"priority"`
	SelfLink          string   `json:"selfLink"`
	SourceRanges      []string `json:"sourceRanges"`
	DestinationRanges []string `json:"destinationRanges"`
	SourceTags        []string `json:"sourceTags,omitempty"`
	TargetTags        []string `json:"targetTags,omitempty"`
}

// GoogleCloudProjectFirewall uses gcloud to generate a list of firewalls
type GoogleCloudProjectFirewall struct{}

func (p *GoogleCloudProjectFirewall) Description() ProcessorDescription {
	return ProcessorDescription{
		Kind: "gcp-firewalls",
		Name: "Google Cloud Project Firewalls",
		Steps: []string{
			"Execute 'yacls --kind={{.Kind}} --project={{.Project}}'",
		},
	}
}

func gcloudTargetsString(targets []gcloudTarget) string {
	ts := []string{}
	for _, t := range targets {
		if len(t.Ports) == 0 {
			ts = append(ts, t.IPProtocol)
			continue
		}
		for _, p := range t.Ports {
			ts = append(ts, fmt.Sprintf("%s:%s", t.IPProtocol, p))
		}
	}
	sort.Strings(ts)
	return strings.Join(ts, ",")
}

func (p *GoogleCloudProjectFirewall) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	a := &Artifact{Metadata: src}
	a.Metadata.ID = c.Project

	project := c.Project
	cmd := exec.Command("gcloud", "compute", "firewall-rules", "list", "--project", project, "--format=json")
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	// klog.Infof("output: %s", stdout)

	full := []gcloudFirewallFull{}
	err = json.Unmarshal(stdout, &full)

	for _, r := range full {
		if r.Disabled {
			continue
		}
		fw := FirewallRuleMeta{
			Name:        r.Name,
			Description: r.Description,
			Logging:     r.LogConfig.Enable,
			Priority:    r.Priority,
			Rule: FirewallRule{
				Sources:      strings.Join(r.SourceRanges, ","),
				Destinations: strings.Join(r.DestinationRanges, ","),
				SourceTags:   strings.Join(r.SourceTags, ","),
				TargetTags:   strings.Join(r.TargetTags, ","),
				Allow:        gcloudTargetsString(r.Allowed),
				Deny:         gcloudTargetsString(r.Denied),
			},
		}

		net := r.Network[strings.LastIndex(r.Network, "/")+1:]
		if net != "default" {
			fw.Rule.Network = net
		}

		switch r.Direction {
		case "INGRESS":
			a.Ingress = append(a.Ingress, fw)
		case "EGRESS":
			a.Egress = append(a.Egress, fw)
		default:
			return nil, fmt.Errorf("unexpected direction: %q", r.Direction)
		}
	}

	// https://cloud.google.com/firewall/docs/firewalls#default_firewall_rules
	a.Ingress = append(a.Ingress, FirewallRuleMeta{
		Name:        "gcp-ingress-fallback",
		Description: "GCP Implied Ingress Fallback",
		Logging:     false,
		Priority:    65535,
		Rule: FirewallRule{
			Sources: "0.0.0.0/0",
			Deny:    "all",
		},
	})

	a.Egress = append(a.Egress, FirewallRuleMeta{
		Name:        "gcp-egress-fallback",
		Description: "GCP Implied Egress Fallback",
		Logging:     false,
		Priority:    65535,
		Rule: FirewallRule{
			Destinations: "0.0.0.0/0",
			Allow:        "all",
		},
	})

	// klog.Infof("full: %+v", full)
	return a, err
}
