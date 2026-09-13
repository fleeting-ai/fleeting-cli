package pi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type File struct {
	Providers map[string]Provider `json:"providers"`
}

type Provider struct {
	BaseURL string         `json:"baseUrl"`
	API     string         `json:"api"`
	APIKey  string         `json:"apiKey,omitempty"`
	Compat  map[string]any `json:"compat,omitempty"`
	Models  []Model        `json:"models"`
}

type Model struct {
	ID             string         `json:"id"`
	Name           string         `json:"name,omitempty"`
	Reasoning      bool           `json:"reasoning,omitempty"`
	ContextWindow  int            `json:"contextWindow,omitempty"`
	MaxTokens      int            `json:"maxTokens,omitempty"`
	SamplingParams map[string]any `json:"samplingParams,omitempty"`
}

type Engine struct {
	ID       string
	Title    string
	Port     int
	Budget   string // thinkingTokenBudgetField
	Provider string
}

var Engines = []Engine{
	{ID: "llamacpp", Title: "llama.cpp", Port: 8080, Budget: "thinking_budget_tokens", Provider: "llamacpp"},
	{ID: "vllm", Title: "vLLM", Port: 8000, Budget: "thinking_token_budget", Provider: "vllm"},
	{ID: "sglang", Title: "SGLang", Port: 30000, Budget: "thinking_budget", Provider: "sglang"},
}

type Draft struct {
	Engine        Engine
	Host          string
	Port          int
	Path          string
	Provider      string
	ModelID       string
	APIKey        string
	ContextWindow int
	MaxTokens     int
	Temperature   float64
	Thinking      bool
}

func Path() string {
	if p := os.Getenv("PI_MODELS_JSON"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".pi", "agent", "models.json")
	}
	return filepath.Join(home, ".pi", "agent", "models.json")
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Providers: map[string]Provider{}}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Providers == nil {
		f.Providers = map[string]Provider{}
	}
	return &f, nil
}

func (d Draft) BaseURL() string {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	path := d.Path
	if path == "" {
		path = "/v1"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	port := d.Port
	if port == 0 {
		port = d.Engine.Port
	}
	return fmt.Sprintf("http://%s:%d%s", host, port, strings.TrimRight(path, "/"))
}

func (d Draft) ProviderEntry() Provider {
	key := strings.TrimSpace(d.APIKey)
	if key == "" {
		key = "local"
	}
	compat := map[string]any{
		"supportsDeveloperRole":    false,
		"supportsReasoningEffort":  false,
		"supportsUsageInStreaming": false,
		"maxTokensField":           "max_tokens",
	}
	if d.Thinking && d.Engine.Budget != "" {
		compat["thinkingTokenBudgetField"] = d.Engine.Budget
	}
	m := Model{
		ID:            d.ModelID,
		Name:          d.ModelID,
		Reasoning:     d.Thinking,
		ContextWindow: d.ContextWindow,
		MaxTokens:     d.MaxTokens,
	}
	if d.Temperature > 0 {
		m.SamplingParams = map[string]any{"temperature": d.Temperature}
	}
	return Provider{
		BaseURL: d.BaseURL(),
		API:     "openai-completions",
		APIKey:  key,
		Compat:  compat,
		Models:  []Model{m},
	}
}

func Merge(f *File, providerName string, p Provider) {
	if f.Providers == nil {
		f.Providers = map[string]Provider{}
	}
	cur, ok := f.Providers[providerName]
	if !ok {
		f.Providers[providerName] = p
		return
	}
	cur.BaseURL = p.BaseURL
	cur.API = p.API
	if p.APIKey != "" {
		cur.APIKey = p.APIKey
	}
	if p.Compat != nil {
		if cur.Compat == nil {
			cur.Compat = map[string]any{}
		}
		for k, v := range p.Compat {
			cur.Compat[k] = v
		}
	}
	for _, nm := range p.Models {
		replaced := false
		for i, em := range cur.Models {
			if em.ID == nm.ID {
				cur.Models[i] = nm
				replaced = true
				break
			}
		}
		if !replaced {
			cur.Models = append(cur.Models, nm)
		}
	}
	f.Providers[providerName] = cur
}

func Save(path string, f *File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func Apply(path string, providerName string, d Draft) error {
	f, err := Load(path)
	if err != nil {
		return err
	}
	Merge(f, providerName, d.ProviderEntry())
	return Save(path, f)
}

func ParseInt(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func ParseFloat(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return n
}
