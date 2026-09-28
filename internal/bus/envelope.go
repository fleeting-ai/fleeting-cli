package bus

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const MaxBody = 32 * 1024

// AgentRules is printed by send/listen/card -h and fleeting bus.
const AgentRules = `At session start, run fleeting card. That is the shared picture: your onboarding, the fleet rules, and the memories you may see.
fyi: do not reply unless you have something necessary.
todo: do the work if it is yours. If order is true, it preempts current non-order work. Then ack.
order: same as an ordered todo. Only a higher rank can send one.
ack: do not reply.
Never invent a recipient. Unknown names fail.
Send with fleeting send --from "$FLEETING_AGENT". Receive with fleeting listen --as "$FLEETING_AGENT". Block. Do not sleep.
A fact the other harnesses need goes in fleeting memory, not in a private note. A standing constraint goes in fleeting rules (rank >= 40).`

type Envelope struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	From    string `json:"from"`
	To      string `json:"to"`
	Team    string `json:"team"`
	Rank    int    `json:"rank"`
	Order   bool   `json:"order"`
	Thread  string `json:"thread"`
	ReplyTo string `json:"reply_to"`
	Body    string `json:"body"`
	At      string `json:"at"`
}

type Record struct {
	Envelope
	ToAgent    string `json:"to_agent"`
	ConsumedAt string `json:"consumed_at,omitempty"`
	ListenedAt string `json:"listened_at,omitempty"`
}

func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func (e Envelope) ShortID() string {
	if len(e.ID) >= 8 {
		return e.ID[:8]
	}
	return e.ID
}

// Frame is the PTY injection line. No trailing Enter.
func (e Envelope) Frame() string {
	var bits []string
	bits = append(bits, e.Action)
	if e.Team != "" {
		bits = append(bits, "team:"+e.Team)
	}
	bits = append(bits, fmt.Sprintf("%s→%s #%s", e.From, e.To, e.ShortID()))
	if e.Order {
		bits = append(bits, "order")
	}
	if e.Order || e.Action == "order" {
		bits = append(bits, fmt.Sprintf("rank=%d", e.Rank))
	}
	if e.Action == "ack" && e.ReplyTo != "" {
		short := e.ReplyTo
		if len(short) > 8 {
			short = short[:8]
		}
		bits = append(bits, "reply="+short)
	}
	return fmt.Sprintf("[fleeting %s] %s", strings.Join(bits, " "), e.Body)
}

func checkBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("body required")
	}
	if len(body) > MaxBody {
		return "", fmt.Errorf("body exceeds 32 KiB")
	}
	for _, r := range body {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return "", fmt.Errorf("body has control characters")
		}
	}
	return body, nil
}
