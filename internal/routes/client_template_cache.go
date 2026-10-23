package routes

import (
	"html/template"
	"sync"
)

var clientTemplateCache sync.Map // map[string]*template.Template

func cachedClientTemplate(key string, files ...string) (*template.Template, error) {
	if v, ok := clientTemplateCache.Load(key); ok {
		return v.(*template.Template), nil
	}
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		return nil, err
	}
	actual, _ := clientTemplateCache.LoadOrStore(key, tmpl)
	return actual.(*template.Template), nil
}

const (
	tplKeyClientHome    = "client-home"
	tplKeyClientList    = "client-list"
	tplKeyClientTag     = "client-tag"
	tplKeyClientAuthor  = "client-author"
	tplKeyClientArticle = "client-article"
	tplKeyClientStatic  = "client-static-page"
	tplKeyClientError   = "client-error"
)

func clientHomeTemplate() (*template.Template, error) {
	return cachedClientTemplate(tplKeyClientHome,
		"public/templates/layouts/client/app.html",
		"public/templates/layouts/client/header.html",
		"public/templates/layouts/client/footer.html",
		"public/templates/partials/client/blog_sidebar.html",
		"public/templates/partials/client/tag_popular_strip.html",
		"public/templates/partials/client/home_ssr.html",
		"public/templates/partials/client/home_ssr_detik.html",
		"public/templates/partials/client/home_ssr_tirtayasa.html",
		"public/templates/pages/client/index.html",
	)
}

func clientListTemplate(pageFile string) (*template.Template, error) {
	return cachedClientTemplate(tplKeyClientList+"-"+pageFile,
		"public/templates/layouts/client/app.html",
		"public/templates/layouts/client/header.html",
		"public/templates/layouts/client/footer.html",
		"public/templates/partials/client/blog_sidebar.html",
		"public/templates/partials/client/topic_posts_list.html",
		"public/templates/partials/client/pagination.html",
		pageFile,
	)
}

func clientTagTemplate() (*template.Template, error) {
	return clientListTemplate("public/templates/pages/client/tag.html")
}

func clientPostsTemplate() (*template.Template, error) {
	return clientListTemplate("public/templates/pages/client/posts.html")
}

func clientCategoryTemplate() (*template.Template, error) {
	return clientListTemplate("public/templates/pages/client/category.html")
}

func clientAuthorTemplate() (*template.Template, error) {
	return clientListTemplate("public/templates/pages/client/author.html")
}

func clientArticleTemplate() (*template.Template, error) {
	return cachedClientTemplate(tplKeyClientArticle,
		"public/templates/layouts/client/app.html",
		"public/templates/layouts/client/header.html",
		"public/templates/layouts/client/footer.html",
		"public/templates/partials/client/blog_sidebar.html",
		"public/templates/partials/client/article_neo_top.html",
		"public/templates/partials/client/article_more_sections.html",
		"public/templates/pages/client/post.html",
	)
}
