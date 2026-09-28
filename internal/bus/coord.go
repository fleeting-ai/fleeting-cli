package bus

import (
	"fmt"
	"strings"
)

func CanAppendRules(rank int) bool { return rank >= 40 }

func CanLogAll(rank int) bool { return rank >= 40 }

func CanAppendOnboarding(w World, as, target string) error {
	src, ok := w.Get(as)
	if !ok {
		return fmt.Errorf("unknown agent %s", as)
	}
	if as == target {
		return nil
	}
	dst, ok := w.Get(target)
	if !ok {
		return fmt.Errorf("unknown agent %s", target)
	}
	if src.Rank > dst.Rank {
		return nil
	}
	return fmt.Errorf("denied: %s cannot append onboarding for %s", as, target)
}

func CanWriteMemory(w World, as, scope string) error {
	if _, ok := w.Get(as); !ok {
		return fmt.Errorf("unknown agent %s", as)
	}
	kind, team, err := ParseScope(scope)
	if err != nil {
		return err
	}
	if kind == "fleet" {
		return nil
	}
	if !w.Member(as, team) {
		return fmt.Errorf("denied: %s is not a member of %s", as, team)
	}
	return nil
}

func CanReadMemory(w World, as, scope string) error {
	src, ok := w.Get(as)
	if !ok {
		return fmt.Errorf("unknown agent %s", as)
	}
	kind, team, err := ParseScope(scope)
	if err != nil {
		return err
	}
	if kind == "fleet" {
		return nil
	}
	if src.Rank >= 40 || w.Member(as, team) {
		return nil
	}
	return fmt.Errorf("denied: %s cannot read %s", as, scope)
}

func CanListAllScopes(rank int) bool { return rank >= 40 }

func SeedCard(name, role string, rank int, peers, teams []string) string {
	return fmt.Sprintf("name: %s\nrole: %s\nrank: %d\npeers: %s\nteams: %s\n\n%s\n",
		name, role, rank, strings.Join(peers, ", "), strings.Join(teams, ", "), AgentRules)
}

func FormatCard(onboard *Doc, rules *Doc, fleet []Memory, fleetN int, teams map[string][]Memory, teamNs map[string]int) string {
	var b strings.Builder
	b.WriteString("== onboarding ==\n")
	if onboard != nil {
		b.WriteString(onboard.Body)
		if !strings.HasSuffix(onboard.Body, "\n") {
			b.WriteByte('\n')
		}
	} else {
		b.WriteString("(none)\n")
	}
	b.WriteString("\n== rules ==\n")
	if rules != nil {
		b.WriteString(rules.Body)
		if !strings.HasSuffix(rules.Body, "\n") {
			b.WriteByte('\n')
		}
	} else {
		b.WriteString("(none)\n")
	}
	writeMem := func(title string, ms []Memory, total int) {
		fmt.Fprintf(&b, "\n== %s ==\n", title)
		if len(ms) == 0 {
			b.WriteString("(none)\n")
			return
		}
		for _, m := range ms {
			fmt.Fprintf(&b, "%s: %s\n", m.Key, m.Body)
		}
		if total > len(ms) {
			fmt.Fprintf(&b, "omitted %d; fleeting memory list\n", total-len(ms))
		}
	}
	writeMem("memories fleet", fleet, fleetN)
	for id, ms := range teams {
		writeMem("memories team:"+id, ms, teamNs[id])
	}
	return b.String()
}
