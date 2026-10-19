package client

import (
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var clientTemplateCache sync.Map // map[string]*template.Template

// InvalidateClientTemplateCache clears parsed client templates so disk changes take effect.
func InvalidateClientTemplateCache() {
	clientTemplateCache.Range(func(key, _ any) bool {
		clientTemplateCache.Delete(key)
		return true
	})
}

var clientFuncs = template.FuncMap{
	"resizeImg": func(src string, w, h int) string {
		src = strings.TrimSpace(src)
		if src == "" || strings.HasPrefix(src, "data:") {
			return src
		}
		u, err := url.Parse(src)
		if err != nil {
			return src
		}
		q := u.Query()
		if w > 0 {
			q.Set("w", strconv.Itoa(w))
		}
		if h > 0 {
			q.Set("h", strconv.Itoa(h))
		}
		u.RawQuery = q.Encode()
		return u.String()
	},
}

// TemplateFuncs returns the FuncMap used when parsing client HTML templates.
func TemplateFuncs() template.FuncMap {
	return clientFuncs
}

