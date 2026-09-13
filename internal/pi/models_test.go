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
