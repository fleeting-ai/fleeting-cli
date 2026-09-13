package pi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	Providers map[string]Provider `json:"providers" yaml:"providers"`
}

type Provider struct {
	BaseURL string         `json:"baseUrl" yaml:"baseUrl"`
	API     string         `json:"api" yaml:"api"`
	APIKey  string         `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Compat  map[string]any `json:"compat,omitempty" yaml:"compat,omitempty"`
	Models  []Model        `json:"models" yaml:"models"`
}

type Model struct {
	ID             string         `json:"id" yaml:"id"`
	Name           string         `json:"name,omitempty" yaml:"name,omitempty"`
	Reasoning      bool           `json:"reasoning,omitempty" yaml:"reasoning,omitempty"`
	ContextWindow  int            `json:"contextWindow,omitempty" yaml:"contextWindow,omitempty"`
	MaxTokens      int            `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`
	SamplingParams map[string]any `json:"samplingParams,omitempty" yaml:"samplingParams,omitempty"`
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
	{ID: "vllm", Title: "vLLM", Port: 8000, Budget: "", Provider: "vllm"},
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
	return ConfigPath("pi")
}

func ConfigPath(kind string, persona ...string) string {
	if kind == "omp" {
		if p := os.Getenv("OMP_MODELS_YML"); p != "" {
			return p
		}
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		name := ""
		if len(persona) > 0 {
			name = strings.TrimSpace(persona[0])
		}
		if name != "" && name != "default" {
			return filepath.Join(home, ".omp", "profiles", name, "agent", "models.yml")
		}
		return filepath.Join(home, ".omp", "agent", "models.yml")
	}
	if p := os.Getenv("PI_MODELS_JSON"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".pi", "agent", "models.json")
	}
	return filepath.Join(home, ".pi", "agent", "models.json")
}

func isYAML(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yml" || ext == ".yaml"
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
	if isYAML(path) {
		err = yaml.Unmarshal(b, &f)
	} else {
		err = json.Unmarshal(b, &f)
	}
	if err != nil {
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
	return d.providerEntry(false)
}

func (d Draft) providerEntry(omp bool) Provider {
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
	// OMP's models.yml schema has no thinkingTokenBudgetField (that's Pi).
	if !omp && d.Thinking && d.Engine.Budget != "" && d.Engine.ID != "vllm" {
		compat["thinkingTokenBudgetField"] = d.Engine.Budget
	}
	m := Model{
		ID:            d.ModelID,
		Name:          d.ModelID,
		Reasoning:     d.Thinking,
		ContextWindow: d.ContextWindow,
		MaxTokens:     d.MaxTokens,
	}
	// samplingParams is valid in Pi models.json, rejected by OMP models.yml schema.
	if !omp && d.Temperature > 0 {
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
	if providerName == "vllm" && cur.Compat != nil {
		delete(cur.Compat, "thinkingTokenBudgetField")
		delete(cur.Compat, "supportsThinkingTokenBudget")
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
	var b []byte
	var err error
	if isYAML(path) {
		var buf strings.Builder
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		err = enc.Encode(f)
		_ = enc.Close()
		b = []byte(buf.String())
	} else {
		b, err = json.MarshalIndent(f, "", "  ")
		if err == nil {
			b = append(b, '\n')
		}
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func Apply(path string, providerName string, d Draft) error {
	return applyAt(path, providerName, d, isYAML(path))
}

func ApplyKind(kind, providerName string, d Draft, persona ...string) error {
	path := ConfigPath(kind, persona...)
	return applyAt(path, providerName, d, kind == "omp" || isYAML(path))
}

func applyAt(path, providerName string, d Draft, omp bool) error {
	f, err := Load(path)
	if err != nil {
		return err
	}
	Merge(f, providerName, d.providerEntry(omp))
	return Save(path, f)
}

type Entry struct {
	Provider string
	ModelID  string
}

func ListEntries(path string) ([]Entry, error) {
	f, err := Load(path)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for name, p := range f.Providers {
		for _, m := range p.Models {
			if m.ID == "" {
				continue
			}
			out = append(out, Entry{Provider: name, ModelID: m.ID})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].ModelID < out[j].ModelID
	})
	return out, nil
}

func LoadEntry(path, provider, modelID string) (Draft, bool) {
	f, err := Load(path)
	if err != nil {
		return Draft{}, false
	}
	p, ok := f.Providers[provider]
	if !ok {
		return Draft{}, false
	}
	var mod Model
	found := false
	for _, m := range p.Models {
		if m.ID == modelID {
			mod = m
			found = true
			break
		}
	}
	if !found {
		return Draft{}, false
	}
	d := Draft{
		Host:          "127.0.0.1",
		Path:          "/v1",
		Provider:      provider,
		ModelID:       mod.ID,
		APIKey:        p.APIKey,
		ContextWindow: mod.ContextWindow,
		MaxTokens:     mod.MaxTokens,
		Thinking:      mod.Reasoning,
	}
	if d.ContextWindow == 0 {
		d.ContextWindow = 32768
	}
	if d.MaxTokens == 0 {
		d.MaxTokens = 8192
	}
	fillHostPort(&d, p.BaseURL)
	for _, e := range Engines {
		if e.Provider == provider {
			d.Engine = e
			break
		}
	}
	if d.Engine.ID == "" {
		d.Engine = Engine{ID: provider, Title: provider, Port: d.Port, Provider: provider}
	}
	if mod.SamplingParams != nil {
		if t, ok := mod.SamplingParams["temperature"].(float64); ok {
			d.Temperature = t
		}
	}
	return d, true
}

func fillHostPort(d *Draft, raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return
	}
	d.Host = u.Hostname()
	if u.Port() != "" {
		d.Port = ParseInt(u.Port(), d.Port)
	}
	if u.Path != "" && u.Path != "/" {
		d.Path = u.Path
	}
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
