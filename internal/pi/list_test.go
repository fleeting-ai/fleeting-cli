package pi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListModelsParsesOpenAIShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen2.5"},{"id":"llama3"},{"id":"qwen2.5"}]}`))
	}))
	t.Cleanup(srv.Close)
	ids, err := ListModels(srv.URL+"/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "qwen2.5" || ids[1] != "llama3" {
		t.Fatalf("%v", ids)
	}
}

func TestListModelsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	t.Cleanup(srv.Close)
	_, err := ListModels(srv.URL+"/v1", "")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("%v", err)
	}
}
