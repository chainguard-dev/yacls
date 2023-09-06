package compare

import (
	"fmt"
	"slices"

	"github.com/chainguard-dev/yacls/v2/pkg/platform"
)

type Change struct {
	Kind     string
	Entity   string
	Mod      string
	FromDate string
	ToDate   string
}

func Summary(from platform.Artifact, to platform.Artifact) ([]Change, error) {
	cs := []Change{}
	fromU := map[string]platform.User{}
	toU := map[string]platform.User{}
	fromGroups := map[string][]string{}
	toGroups := map[string][]string{}

	fromGroupPerms := map[string][]string{}
	toGroupPerms := map[string][]string{}

	kind := to.Metadata.Kind
	fromDate := from.Metadata.SourceDate
	toDate := to.Metadata.SourceDate

	for _, u := range from.Users {
		fromU[u.Account] = u
	}

	for _, g := range from.Groups {
		fromGroups[g.Name] = g.Members
		fromGroupPerms[g.Name] = g.Permissions
	}

	for _, u := range to.Users {
		toU[u.Account] = u
		_, exists := fromU[u.Account]
		if !exists {
			cs = append(cs, Change{Kind: kind, Entity: u.Account, Mod: "add user", FromDate: fromDate, ToDate: toDate})
			continue
		}
	}

	for _, g := range to.Groups {
		toGroups[g.Name] = g.Members
		toGroupPerms[g.Name] = g.Permissions
	}

	for acct, fu := range fromU {
		tu, exists := toU[acct]
		if !exists {
			cs = append(cs, Change{Kind: kind, Entity: fu.Account, Mod: "remove user", FromDate: fromDate, ToDate: toDate})
			continue
		}

		for _, p := range fu.Permissions {
			if !slices.Contains(tu.Permissions, p) {
				cs = append(cs, Change{Kind: kind, Entity: fu.Account, Mod: fmt.Sprintf("remove permission: %s", p), FromDate: fromDate, ToDate: toDate})
			}
		}

		for _, p := range tu.Permissions {
			if !slices.Contains(fu.Permissions, p) {
				cs = append(cs, Change{Kind: kind, Entity: fu.Account, Mod: fmt.Sprintf("add permission: %s", p), FromDate: fromDate, ToDate: toDate})
			}
		}
	}

	for name, members := range fromGroups {
		for _, m := range members {
			if !slices.Contains(toGroups[name], m) {
				cs = append(cs, Change{Kind: kind, Entity: m, Mod: fmt.Sprintf("left group: %s", name), FromDate: fromDate, ToDate: toDate})
			}
		}
	}

	for name, members := range toGroups {
		for _, m := range members {
			if !slices.Contains(fromGroups[name], m) {
				cs = append(cs, Change{Kind: kind, Entity: m, Mod: fmt.Sprintf("joined group: %s", name), FromDate: fromDate, ToDate: toDate})
			}
		}
	}

	for g, ps := range fromGroupPerms {
		for _, p := range ps {
			if !slices.Contains(toGroupPerms[g], p) {
				cs = append(cs, Change{Kind: kind, Entity: g, Mod: fmt.Sprintf("lost permission: %q", p), FromDate: fromDate, ToDate: toDate})
			}
		}
	}

	for g, ps := range toGroupPerms {
		for _, p := range ps {
			if !slices.Contains(fromGroupPerms[g], p) {
				cs = append(cs, Change{Kind: kind, Entity: g, Mod: fmt.Sprintf("gained permission: %q", p), FromDate: fromDate, ToDate: toDate})
			}
		}
	}

	return cs, nil
}
