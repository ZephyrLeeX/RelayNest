// Package webui 提供随服务端二进制交付的空管理入口。
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// 先在 web 目录执行 npm ci && npm run build，再编译或测试服务端。
// 不提交生成物；缺少产物时让编译明确失败，避免交付空页面。
//
//go:embed dist
var content embed.FS

// New 返回静态文件处理器；调用方只挂载根页面和 /assets/。
func New() http.Handler {
	files, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}
	static := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		} else if !strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		info, err := fs.Stat(files, name)
		// 不提供目录列表，也不暴露源文件或不存在路径的首页回退。
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		// 入口每次重新验证，避免浏览器跨版本复用旧资源引用。
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		static.ServeHTTP(w, r)
	})
}
