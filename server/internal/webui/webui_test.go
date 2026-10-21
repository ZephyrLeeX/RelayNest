package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedFiles(t *testing.T) {
	h := New()
	index, err := content.ReadFile("dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "RelayNest 管理") || strings.Contains(string(index), "/src/") {
		t.Fatal("首页不是构建后的管理入口")
	}
	entries, err := content.ReadDir("dist/assets")
	if err != nil || len(entries) == 0 {
		t.Fatalf("缺少构建资源: %v", err)
	}
	for _, entry := range entries {
		path := "/assets/" + entry.Name()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		want, err := content.ReadFile("dist" + path)
		if err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || w.Body.String() != string(want) {
			t.Errorf("%s 未返回内嵌资源: %d", path, w.Code)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s 缺少类型保护", path)
		}
	}
	for _, path := range []string{"/assets/", "/assets/missing.js", "/src/main.ts", "/index.html", "/assets/../index.html"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Errorf("%s = %d, want 404", path, w.Code)
		}
	}
}
