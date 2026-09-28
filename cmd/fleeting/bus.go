package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fleeting-ai/fleeting-cli/internal/bus"
	"github.com/fleeting-ai/fleeting-cli/internal/listen"
)

func cmdHelp(kind string) bool {
	for _, a := range os.Args[1:] {
		if a == "-h" || a == "--help" {
			fmt.Print(bus.AgentRules)
			fmt.Print("\n\n")
			switch kind {
			case "send":
				fmt.Print("usage: fleeting send --from NAME (--to NAME|--team ID) [--action fyi|todo|order|ack] [--reply-to ID] [--thread T] -- BODY\n")
			case "listen":
				fmt.Print("usage: fleeting listen --as NAME [--once]\n")
			case "card":
				fmt.Print("usage: fleeting card --as NAME\n")
			}
			return true
		}
	}
	return false
}

func afterDash(args []string) string {
	for i, a := range args {
		if a == "--" {
			return strings.TrimSpace(strings.Join(args[i+1:], " "))
		}
	}
	return ""
}

func flagVal(args []string, names ...string) string {
	for i := 0; i < len(args); i++ {
		for _, n := range names {
			if args[i] == n && i+1 < len(args) && args[i+1] != "--" {
				return args[i+1]
			}
		}
	}
	return ""
}

func hasFlag(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

func dialOrDie() *listen.Client {
	cl, err := listen.Dial()
	if err != nil {
		fatal(fmt.Errorf("no daemon: %w", err))
	}
	return cl
}

func runSend(args []string) {
	if cmdHelp("send") {
		return
	}
	from := flagVal(args, "--from")
	to := flagVal(args, "--to")
	team := flagVal(args, "--team")
	action := flagVal(args, "--action")
	reply := flagVal(args, "--reply-to")
	thread := flagVal(args, "--thread")
	body := afterDash(args)
	if from == "" || body == "" {
		fatal(fmt.Errorf("usage: fleeting send --from NAME (--to NAME|--team ID) -- BODY"))
	}
	cl := dialOrDie()
	defer cl.Close()
	if err := cl.RouteMail(bus.Send{From: from, To: to, Team: team, Action: action, Body: body, ReplyTo: reply, Thread: thread}); err != nil {
		fatal(err)
	}
}

func runListen(args []string) {
	if cmdHelp("listen") {
		return
	}
	as := flagVal(args, "--as")
	if as == "" {
		fatal(fmt.Errorf("usage: fleeting listen --as NAME [--once]"))
	}
	once := hasFlag(args, "--once")
	cl := dialOrDie()
	defer cl.Close()
	for {
		p, err := cl.Coord(listen.Packet{Kind: "listen", Name: as})
		if err != nil {
			fatal(err)
		}
		if len(p.Mail) > 0 {
			fmt.Println(p.Mail[0].Frame())
			if once {
				return
			}
			continue
		}
		if once {
			os.Exit(0)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func runLog(args []string) {
	as := flagVal(args, "--as")
	team := flagVal(args, "--team")
	last := 20
	if v := flagVal(args, "--last"); v != "" {
		fmt.Sscanf(v, "%d", &last)
	}
	cl := dialOrDie()
	defer cl.Close()
	p, err := cl.Coord(listen.Packet{Kind: "log", Name: as, From: as, Team: team, Limit: last, All: hasFlag(args, "--all")})
	if err != nil {
		fatal(err)
	}
	for _, m := range p.Mail {
		fmt.Println(m.Frame())
	}
}

func runCard(args []string) {
	if cmdHelp("card") {
		return
	}
	as := flagVal(args, "--as")
	if as == "" {
		fatal(fmt.Errorf("usage: fleeting card --as NAME"))
	}
	cl := dialOrDie()
	defer cl.Close()
	p, err := cl.Coord(listen.Packet{Kind: "card", Name: as})
	if err != nil {
		fatal(err)
	}
	fmt.Print(p.Body)
}

func runRules(args []string) {
	cl := dialOrDie()
	defer cl.Close()
	if len(args) == 0 {
		p, err := cl.Coord(listen.Packet{Kind: "rules_get"})
		if err != nil {
			fatal(err)
		}
		fmt.Print(p.Body)
		if p.Body != "" && !strings.HasSuffix(p.Body, "\n") {
			fmt.Println()
		}
		return
	}
	switch args[0] {
	case "set":
		from := flagVal(args, "--from")
		body := afterDash(args)
		if from == "" || body == "" {
			fatal(fmt.Errorf("usage: fleeting rules set --from NAME -- BODY"))
		}
		if _, err := cl.Coord(listen.Packet{Kind: "rules_set", From: from, Body: body}); err != nil {
			fatal(err)
		}
	case "log":
		p, err := cl.Coord(listen.Packet{Kind: "rules_log"})
		if err != nil {
			fatal(err)
		}
		fmt.Print(p.Body)
	default:
		fatal(fmt.Errorf("usage: fleeting rules | fleeting rules set --from NAME -- BODY | fleeting rules log"))
	}
}

func runMemory(args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("usage: fleeting memory list|get|set|log ..."))
	}
	cl := dialOrDie()
	defer cl.Close()
	switch args[0] {
	case "list":
		as := flagVal(args, "--as")
		scope := ""
		if pos := positional(args[1:]); len(pos) > 0 {
			scope = pos[0]
		}
		p, err := cl.Coord(listen.Packet{Kind: "memory_list", From: as, Name: scope})
		if err != nil {
			fatal(err)
		}
		fmt.Print(p.Body)
	case "get":
		as := flagVal(args, "--as")
		pos := positional(args[1:])
		if as == "" || len(pos) < 2 {
			fatal(fmt.Errorf("usage: fleeting memory get --as NAME SCOPE KEY"))
		}
		p, err := cl.Coord(listen.Packet{Kind: "memory_get", From: as, Name: pos[0], Key: pos[1]})
		if err != nil {
			fatal(err)
		}
		fmt.Print(p.Body)
	case "set":
		from := flagVal(args, "--from")
		pos := positional(args[1:])
		body := afterDash(args)
		if from == "" || len(pos) < 2 || body == "" {
			fatal(fmt.Errorf("usage: fleeting memory set --from NAME SCOPE KEY -- BODY"))
		}
		if _, err := cl.Coord(listen.Packet{Kind: "memory_set", From: from, Name: pos[0], Key: pos[1], Body: body}); err != nil {
			fatal(err)
		}
	case "log":
		as := flagVal(args, "--as")
		pos := positional(args[1:])
		if as == "" || len(pos) < 2 {
			fatal(fmt.Errorf("usage: fleeting memory log --as NAME SCOPE KEY"))
		}
		p, err := cl.Coord(listen.Packet{Kind: "memory_log", From: as, Name: pos[0], Key: pos[1]})
		if err != nil {
			fatal(err)
		}
		fmt.Print(p.Body)
	default:
		fatal(fmt.Errorf("usage: fleeting memory list|get|set|log ..."))
	}
}

func positional(args []string) []string {
	var out []string
	skip := false
	for i := 0; i < len(args); i++ {
		if skip {
			skip = false
			continue
		}
		if args[i] == "--" {
			break
		}
		if strings.HasPrefix(args[i], "--") {
			if args[i] != "--all" && args[i] != "--once" {
				skip = true
			}
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func runBus(args []string) {
	fmt.Println(bus.AgentRules)
	cl, err := listen.Dial()
	if err != nil {
		fmt.Printf("broker: down (no daemon)\nstore: down (no daemon)\n")
		return
	}
	defer cl.Close()
	st, err := cl.QueryStatus()
	if err != nil {
		fmt.Printf("broker: down\nstore: down\n")
		return
	}
	fmt.Printf("broker: %s %s\n", nz(st.Bus, "down"), st.BusURL)
	fmt.Printf("store: %s %s\n", nz(st.Store, "down"), st.StoreURL)
}

func nz(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
