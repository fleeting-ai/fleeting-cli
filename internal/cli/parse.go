package cli

import (
	"fmt"
	"strings"
)

// Action is the parsed fleeting command line (screen-like flags + verbs).
type Action struct {
	Kind    string // up, attach, list, detach, daemon, down, onboard, declare, send, listen, log, card, rules, memory, bus, help
	Steal   bool
	GUID    string
	Go      bool
	Declare []string
	Args    []string
}

func Parse(args []string) (Action, error) {
	var a Action
	var verb string
	var positional []string
	hasR, hasD, hasLS := false, false, false

	for i := 0; i < len(args); i++ {
		s := args[i]
		switch s {
		case "-h", "--help", "help":
			a.Kind = "help"
			return a, nil
		case "-ls", "--list", "ls", "list":
			hasLS = true
		case "-r", "--attach", "attach":
			hasR = true
		case "-d", "--detach":
			hasD = true
		case "detach":
			if verb == "" {
				verb = "detach"
			}
			hasD = true
		case "-X", "--quit", "down":
			verb = "down"
		case "daemon":
			verb = "daemon"
		case "up":
			verb = "up"
		case "onboard":
			verb = "onboard"
		case "declare":
			a.Kind = "declare"
			a.Declare = append([]string{}, args[i+1:]...)
			return a, nil
		case "send", "listen", "log", "card", "rules", "memory", "bus":
			a.Kind = s
			a.Args = append([]string{}, args[i+1:]...)
			return a, nil
		case "r":
			hasR = true
		case "--go":
			a.Go = true
		default:
			if strings.HasPrefix(s, "-") {
				return a, fmt.Errorf("unknown flag %s (try fleeting -h)", s)
			}
			positional = append(positional, s)
		}
	}

	switch {
	case verb == "onboard" || verb == "daemon" || verb == "down":
		a.Kind = verb
	case hasLS:
		a.Kind = "list"
	case hasR && hasD:
		a.Kind = "attach"
		a.Steal = true
	case hasR:
		a.Kind = "attach"
	case verb == "detach" || (hasD && verb == ""):
		a.Kind = "detach"
	case verb == "up":
		a.Kind = "up"
		a.Steal = hasD
	default:
		a.Kind = "up"
		a.Steal = hasD
	}
	if a.Kind == "attach" && len(positional) > 0 {
		a.GUID = positional[0]
	}
	return a, nil
}
