package handler

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template/parse"

	"github.com/gin-gonic/gin"

	clienttpl "github.com/Bebii-Digital/bebii-blog/internal/helper/client"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/routes/dto"
)

var (
	siteLayoutDefineOpenRe  = regexp.MustCompile(`^\{\{-?\s*define\s+"[^"]+"\s*-?\}\}\s*$`)
	siteLayoutDefineCloseRe = regexp.MustCompile(`^\{\{-?\s*end\s*-?\}\}\s*$`)
)

var clientTemplateCacheInvalidate func()

// SetClientTemplateCacheInvalidate wires cache invalidation from routes (avoids import cycle).
func SetClientTemplateCacheInvalidate(fn func()) {
	clientTemplateCacheInvalidate = fn
}

// ApplyFragmentToFile writes fragment textarea content to the corresponding client template file.
func (h *SiteLayoutHandler) ApplyFragmentToFile(c *gin.Context) {
	var req dto.ApplySiteLayoutFragmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data permintaan tidak valid"})
		return
	}

	fragment := normalizeSiteLayoutFragment(req.Fragment)
	if fragment == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fragment tidak valid"})
		return
	}

	content := req.Content
	if strings.TrimSpace(content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Konten fragment tidak boleh kosong"})
		return
	}

	path, err := applyFragmentToTemplateFile(fragment, content)
	if err != nil {
		writeSiteLayoutApplyError(c, err)
		return
	}

	if clientTemplateCacheInvalidate != nil {
		clientTemplateCacheInvalidate()
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"fragment": fragment,
		"path":     path,
	})
}

// ApplyLayout writes all non-empty fragments of a layout to disk and marks it as used.
func (h *SiteLayoutHandler) ApplyLayout(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID layout tidak valid"})
		return
	}

	layout, err := h.svc.Get(uint(id))
	if err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Layout tidak ditemukan"})
		return
	}

	applied := make([]string, 0, len(siteLayoutFragmentKeys))
	paths := make([]string, 0, len(siteLayoutFragmentKeys))
	for _, fragment := range siteLayoutFragmentKeys {
		content := siteLayoutFragmentField(layout, fragment)
		if strings.TrimSpace(content) == "" {
			continue
		}
		path, applyErr := applyFragmentToTemplateFile(fragment, content)
		if applyErr != nil {
			writeSiteLayoutApplyError(c, applyErr)
			return
		}
		applied = append(applied, fragment)
		paths = append(paths, path)
	}

	if len(applied) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Layout tidak memiliki fragment yang bisa diterapkan"})
		return
	}

	updated, err := h.svc.SetUsedExclusive(uint(id))
	if err != nil {
		if msg, code := siteLayoutServiceError(err); code != 0 {
			c.JSON(code, gin.H{"error": msg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengaktifkan layout"})
		return
	}

	if clientTemplateCacheInvalidate != nil {
		clientTemplateCacheInvalidate()
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":             true,
		"layout":         updated,
		"applied":        applied,
		"paths":          paths,
		"fragments_count": len(applied),
	})
}

func writeSiteLayoutApplyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errSiteLayoutApplyPath):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, errSiteLayoutApplyMissing):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, errSiteLayoutApplyInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

func siteLayoutFragmentField(row *entity.SiteLayout, fragment string) string {
	if row == nil {
		return ""
	}
	switch fragment {
	case "header":
		return row.Header
	case "footer":
		return row.Footer
	case "home_body":
		return row.HomeBody
	case "article_body":
		return row.ArticleBody
	case "sidebar":
		return row.Sidebar
	case "rss":
		return row.RSS
	case "author_profile":
		return row.AuthorProfile
	case "tag":
		return row.Tag
	case "kategori":
		return row.Kategori
	case "search_page":
		return row.SearchPage
	default:
		return ""
	}
}

var (
	errSiteLayoutApplyPath    = errors.New("path template tidak valid")
	errSiteLayoutApplyMissing = errors.New("file template tidak ditemukan")
	errSiteLayoutApplyInvalid = errors.New("konten template tidak valid")
)

func applyFragmentToTemplateFile(fragment, content string) (string, error) {
	relPath, ok := siteLayoutFragmentRefFiles[fragment]
	if !ok {
		return "", fmt.Errorf("%w: fragment %q", errSiteLayoutApplyPath, fragment)
	}

	clean := filepath.ToSlash(filepath.Clean(relPath))
	if !strings.HasPrefix(clean, "public/templates/") {
		return "", fmt.Errorf("%w: %s", errSiteLayoutApplyPath, relPath)
	}

	existing, readErr := os.ReadFile(clean)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", fmt.Errorf("%w: %s", errSiteLayoutApplyMissing, clean)
		}
		return "", readErr
	}
	existingContent := string(existing)

	if err := siteLayoutValidateIncomingVars(fragment, existingContent, content); err != nil {
		return "", err
	}

	var merged string
	var err error

	if fragment == "sidebar" {
		merged = strings.TrimSpace(content)
	} else {
		merged, err = mergeSingleDefineTemplate(existingContent, content)
		if err != nil {
			return "", err
		}
	}

	if _, parseErr := template.New("apply-check").Funcs(clienttpl.TemplateFuncs()).Parse(merged); parseErr != nil {
		return "", fmt.Errorf("%w: %v", errSiteLayoutApplyInvalid, parseErr)
	}

	if err := os.WriteFile(clean, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return clean, nil
}

func mergeSingleDefineTemplate(existing, newContent string) (string, error) {
	existing = strings.TrimRight(existing, "\r\n")
	lines := strings.Split(existing, "\n")

	openIdx := -1
	closeIdx := -1
	for i, line := range lines {
		if openIdx < 0 && siteLayoutDefineOpenRe.MatchString(strings.TrimRight(line, "\r")) {
			openIdx = i
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if siteLayoutDefineCloseRe.MatchString(strings.TrimRight(lines[i], "\r")) {
			closeIdx = i
			break
		}
	}
	if openIdx < 0 || closeIdx < 0 || closeIdx <= openIdx {
		return "", fmt.Errorf("%w: file template tidak memiliki blok define/end", errSiteLayoutApplyInvalid)
	}

	inner := stripSingleDefineWrappers(newContent)
	inner = strings.TrimRight(inner, "\r\n")

	openLine := strings.TrimRight(lines[openIdx], "\r")
	closeLine := strings.TrimRight(lines[closeIdx], "\r")

	var b strings.Builder
	b.WriteString(openLine)
	b.WriteByte('\n')
	b.WriteString(inner)
	if inner != "" {
		b.WriteByte('\n')
	}
	b.WriteString(closeLine)
	b.WriteByte('\n')
	return b.String(), nil
}

func stripSingleDefineWrappers(content string) string {
	s := strings.TrimSpace(content)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	start := 0
	end := len(lines)
	strippedDefine := false
	if len(lines) > 0 && siteLayoutDefineOpenRe.MatchString(strings.TrimRight(lines[0], "\r")) {
		start = 1
		strippedDefine = true
	}
	if strippedDefine && end > start && siteLayoutDefineCloseRe.MatchString(strings.TrimRight(lines[end-1], "\r")) {
		end = end - 1
	}
	if start >= end {
		return strings.TrimSpace(s)
	}
	return strings.Join(lines[start:end], "\n")
}

func siteLayoutValidateIncomingVars(fragment, existingFile, incoming string) error {
	allowed, err := siteLayoutAllowedRootVars(fragment, existingFile)
	if err != nil {
		return err
	}

	incomingVars, err := siteLayoutCollectRootVars(incoming)
	if err != nil {
		return fmt.Errorf("%w: %v", errSiteLayoutApplyInvalid, err)
	}
	if len(incomingVars) == 0 {
		return nil
	}

	var unknown []string
	for v := range incomingVars {
		if _, ok := allowed[v]; !ok {
			unknown = append(unknown, v)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%w: variabel template tidak dikenal: %s", errSiteLayoutApplyInvalid, strings.Join(unknown, ", "))
}

func siteLayoutCollectRootVars(src string) (map[string]struct{}, error) {
	t, err := template.New("vars").Funcs(clienttpl.TemplateFuncs()).Parse(src)
	if err != nil {
		return nil, err
	}
	vars := make(map[string]struct{})
	for _, tmpl := range t.Templates() {
		if tmpl.Tree != nil && tmpl.Tree.Root != nil {
			siteLayoutWalkTemplateVars(tmpl.Tree.Root, vars)
		}
	}
	return vars, nil
}

func siteLayoutWalkTemplateVars(n parse.Node, vars map[string]struct{}) {
	switch x := n.(type) {
	case *parse.ListNode:
		for _, node := range x.Nodes {
			siteLayoutWalkTemplateVars(node, vars)
		}
	case *parse.ActionNode:
		siteLayoutWalkTemplatePipe(x.Pipe, vars)
	case *parse.IfNode:
		siteLayoutWalkTemplatePipe(x.Pipe, vars)
		siteLayoutWalkTemplateVars(x.List, vars)
		if x.ElseList != nil {
			siteLayoutWalkTemplateVars(x.ElseList, vars)
		}
	case *parse.RangeNode:
		siteLayoutWalkTemplatePipe(x.Pipe, vars)
		siteLayoutWalkTemplateVars(x.List, vars)
		if x.ElseList != nil {
			siteLayoutWalkTemplateVars(x.ElseList, vars)
		}
	case *parse.WithNode:
		siteLayoutWalkTemplatePipe(x.Pipe, vars)
		siteLayoutWalkTemplateVars(x.List, vars)
	}
}

func siteLayoutWalkTemplatePipe(p *parse.PipeNode, vars map[string]struct{}) {
	if p == nil {
		return
	}
	for _, cmd := range p.Cmds {
		siteLayoutWalkTemplateCmd(cmd, vars)
	}
}

func siteLayoutWalkTemplateCmd(cmd parse.Node, vars map[string]struct{}) {
	switch x := cmd.(type) {
	case *parse.FieldNode:
		if len(x.Ident) > 0 {
			root := x.Ident[0]
			if root == "$" && len(x.Ident) > 1 {
				root = x.Ident[1]
			}
			if root != "" && root != "$" {
				vars[root] = struct{}{}
			}
		}
	case *parse.ChainNode:
		if fn, ok := x.Node.(*parse.FieldNode); ok && len(fn.Ident) > 0 {
			vars[fn.Ident[0]] = struct{}{}
		}
	case *parse.CommandNode:
		for _, arg := range x.Args {
			siteLayoutWalkTemplateCmd(arg, vars)
		}
	}
}
