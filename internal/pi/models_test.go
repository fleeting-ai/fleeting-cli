package pi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeKeepsExistingProvider(t *testing.T) {
	f := &File{Providers: map[string]Provider{
		"openai": {BaseURL: "https://api.openai.com/v1", API: "openai-completions", Models: []Model{{ID: "gpt-4"}}},
	}}
	d := Draft{
		Engine:        Engines[0],
		Host:          "127.0.0.1",
		Port:          8080,
		Path:          "/v1",
		ModelID:       "qwen2.5",
		ContextWindow: 32768,
		MaxTokens:     8192,
	}
	Merge(f, "llamacpp", d.ProviderEntry())
	if _, ok := f.Providers["openai"]; !ok {
		t.Fatal("wiped openai")
	}
	if f.Providers["llamacpp"].BaseURL != "http://127.0.0.1:8080/v1" {
		t.Fatalf("url %s", f.Providers["llamacpp"].BaseURL)
	}
	if f.Providers["llamacpp"].APIKey != "local" {
		t.Fatal("dummy key")
	}
}

func TestMergeReplacesSameModelID(t *testing.T) {
	f := &File{Providers: map[string]Provider{
		"vllm": {Models: []Model{{ID: "llama", ContextWindow: 8}}},
	}}
	d := Draft{Engine: Engines[1], ModelID: "llama", ContextWindow: 128000, Host: "127.0.0.1", Port: 8000, Path: "/v1"}
	Merge(f, "vllm", d.ProviderEntry())
	if len(f.Providers["vllm"].Models) != 1 || f.Providers["vllm"].Models[0].ContextWindow != 128000 {
		t.Fatalf("%+v", f.Providers["vllm"].Models)
	}
}

func TestApplyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "models.json")
	if err := os.WriteFile(p, []byte(`{"providers":{"keep":{"baseUrl":"https://x","api":"openai-completions","models":[{"id":"a"}]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := Draft{Engine: Engines[2], Host: "10.0.0.2", Port: 30000, Path: "/v1", ModelID: "local-r1", Thinking: true, Temperature: 0.7, ContextWindow: 65536, MaxTokens: 4096}
	if err := Apply(p, "sglang", d); err != nil {
		t.Fatal(err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Providers["keep"]; !ok {
		t.Fatal("lost keep")
	}
	sg := out.Providers["sglang"]
	if sg.Compat["thinkingTokenBudgetField"] != "thinking_budget" {
		t.Fatalf("compat %+v", sg.Compat)
	}
	if sg.Models[0].SamplingParams["temperature"] != 0.7 {
		t.Fatalf("temp %+v", sg.Models[0].SamplingParams)
	}
}

func TestVLLMOmitsThinkingTokenBudget(t *testing.T) {
	d := Draft{
		Engine:        Engines[1],
		Host:          "127.0.0.1",
		Port:          8000,
		Path:          "/v1",
		ModelID:       "local",
		Thinking:      true,
		ContextWindow: 32768,
		MaxTokens:     8192,
	}
	p := d.ProviderEntry()
	if _, ok := p.Compat["thinkingTokenBudgetField"]; ok {
		t.Fatalf("vLLM V2 rejects thinking_token_budget, got %+v", p.Compat)
	}
	if !p.Models[0].Reasoning {
		t.Fatal("reasoning should still be set")
	}
	f := &File{Providers: map[string]Provider{
		"vllm": {Compat: map[string]any{"thinkingTokenBudgetField": "thinking_token_budget"}},
	}}
	Merge(f, "vllm", p)
	if _, ok := f.Providers["vllm"].Compat["thinkingTokenBudgetField"]; ok {
		t.Fatal("merge should strip leftover budget field")
	}
}

func TestYAMLRoundTripOMP(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "models.yml")
	t.Setenv("OMP_MODELS_YML", p)
	d := Draft{Engine: Engines[1], Host: "127.0.0.1", Port: 8000, Path: "/v1", ModelID: "qwen", ContextWindow: 32768, MaxTokens: 8192}
	if err := ApplyKind("omp", "vllm", d); err != nil {
		t.Fatal(err)
	}
	ents, err := ListEntries(ConfigPath("omp"))
	if err != nil || len(ents) != 1 || ents[0].Provider != "vllm" || ents[0].ModelID != "qwen" {
		t.Fatalf("%v %+v", err, ents)
	}
	got, ok := LoadEntry(p, "vllm", "qwen")
	if !ok || got.Host != "127.0.0.1" || got.Port != 8000 {
		t.Fatalf("load %+v ok=%v", got, ok)
	}
	d.ModelID = "other"
	if err := ApplyKind("omp", "vllm", d); err != nil {
		t.Fatal(err)
	}
	ents, _ = ListEntries(p)
	if len(ents) != 2 {
		t.Fatalf("want 2 models, got %+v", ents)
	}
}
