package platform

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"k8s.io/klog/v2"
)

// gcloudFirewallFull is what is returned by "gcloud firewalls list"
type gcloudFirewallFull struct {
	Allowed []struct {
		IPProtocol string   `json:"IPProtocol"`
		Ports      []string `json:"ports"`
	} `json:"allowed"`
	CreationTimestamp string `json:"creationTimestamp"`
	Description       string `json:"description"`
	Direction         string `json:"direction"`
	Disabled          bool   `json:"disabled"`
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	LogConfig         struct {
		Enable bool `json:"enable"`
	} `json:"logConfig"`
	Name         string   `json:"name"`
	Network      string   `json:"network"`
	Priority     int      `json:"priority"`
	SelfLink     string   `json:"selfLink"`
	SourceRanges []string `json:"sourceRanges"`
	TargetTags   []string `json:"targetTags,omitempty"`
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

func (p *GoogleCloudProjectFirewall) Process(c Config) (*Artifact, error) {
	src, err := NewSourceFromConfig(c, p)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	a := &Artifact{Metadata: src}
	a.Metadata.ID = project

	project := c.Project
	cmd := exec.Command("gcloud", "compute", "firewall-rules", "list", "--project", project, "--format=json")
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s: %w\nstderr: %s", cmd, err, ee.Stderr)
		}
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}

	//klog.Infof("output: %s", stdout)

	full := []gcloudFirewallFull{}
	err = json.Unmarshal(stdout, &full)

	for _, r := range full {
		fw := FirewallRule{
			Description:  r.Description,
			Direction:    r.Direction,
			Logging:      r.LogConfig.Enable,
			SourceRanges: r.SourceRanges,
			Targets:      r.TargetTags,
		}
		net := r.Network[strings.LastIndex(r.Network, "/")+1:]
		if net != "default" {
			fw.Network = net
		}

		for _, a := range r.Allowed {
			sot := SourceOrTarget{Protocol: a.IPProtocol, Ports: a.Ports}
			fw.Allow = append(fw.Allow, sot)
		}

		switch r.Direction {
		case "INGRESS":
			a.Firewall.Ingress = append(a.Firewall.Ingress, fw)
		case "EGRESS":
			a.Firewall.Egress = append(a.Firewall.Egress, fw)
		default:
			return nil, fmt.Errorf("unexpected direction: %q", r.Direction)
		}
	}

	klog.Infof("full: %+v", full)
	return a, err
}
