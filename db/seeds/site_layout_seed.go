package seeds

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"

	clienttpl "github.com/Bebii-Digital/bebii-blog/internal/helper/client"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
)

// Paths written by Admin "Apply Layout". Seed must re-apply the active layout after upsert
// so deploy/git checkout of repo templates does not silently switch the live theme.
var siteLayoutLiveFragmentFiles = map[string]string{
	"header":         "public/templates/layouts/client/header.html",
	"footer":         "public/templates/layouts/client/footer.html",
	"home_body":      "public/templates/pages/client/index.html",
	"article_body":   "public/templates/pages/client/post.html",
	"sidebar":        "public/templates/partials/client/blog_sidebar.html",
	"rss":            "public/templates/pages/client/rss_feeds.html",
	"author_profile": "public/templates/pages/client/author.html",
	"tag":            "public/templates/pages/client/tag.html",
	"kategori":       "public/templates/pages/client/category.html",
	"search_page":    "public/templates/pages/client/posts.html",
}

var siteLayoutLiveFragmentOrder = []string{
	"header", "footer", "home_body", "article_body", "sidebar",
	"rss", "author_profile", "tag", "kategori", "search_page",
}

const siteLayoutAIPrompt = `==================================================
SOURCE OF TRUTH
==================================================

The screenshot is the ONLY source of truth for:

- Layout
- Structure
- Sections
- Visual hierarchy
- Component placement
- Navigation
- Typography hierarchy
- Content organization
- Responsive behavior

The provided Go Template variables are NOT a layout reference.

They exist ONLY as optional data sources.

Do NOT redesign, restructure, add, remove, merge, split, or reorder sections to fit the variables.

If the screenshot contains a section that has no matching variable:

USE HARDCODED PLACEHOLDER CONTENT.

Never remove a screenshot section merely because a variable does not exist.

Never create new sections merely because variables exist.

Screenshot structure always wins.

==================================================
VARIABLE USAGE POLICY
==================================================

Variables are optional.

Use a variable ONLY when:

1. The screenshot contains equivalent content.
2. The variable naturally maps to that content.

If no suitable variable exists:

USE HARDCODED CONTENT.

It is acceptable for 80-90% of the page to be hardcoded if the screenshot requires it.

Do NOT force variables into unrelated sections.

Do NOT create UI elements solely to consume variables.

Unused variables are acceptable.

When a screenshot element clearly corresponds to an available variable:

USE THE VARIABLE.

Examples:

Screenshot logo
→ {{ .Site.Name }}

Screenshot article title
→ {{ .Title }}

Screenshot article list
→ {{ range .Posts }}

Screenshot footer copyright
→ {{ .Footer.Copyright }}

Otherwise use hardcoded placeholder content.

Prefer variables only when there is an obvious semantic match.

Before writing code:

Step 1:
Reconstruct the screenshot layout.

Step 2:
Create the HTML structure matching the screenshot.

Step 3:
Identify where available variables can replace static content.

Step 4:
Leave all remaining content hardcoded.

Do NOT perform these steps in reverse order.

The goal is NOT:

"Fit the screenshot into the provided CMS structure."

The goal IS:

"Recreate the screenshot faithfully and then inject variables only where appropriate."

If a conflict exists between:

A. Screenshot accuracy
B. Variable utilization

Always choose screenshot accuracy.

Treat the screenshot as the frontend specification.

Treat the Go Template variables as a backend API.

Never allow the backend API to influence the frontend design.

//reference

`

var siteLayoutDefaultFragmentFiles = map[string]string{
	"header":         "header.html",
	"footer":         "footer.html",
	"home_body":      "home_body.html",
	"article_body":   "article_body.html",
	"sidebar":        "sidebar.html",
	"rss":            "rss.html",
	"author_profile": "author_profile.html",
	"tag":            "tag.html",
	"kategori":       "kategori.html",
	"search_page":    "search_page.html",
}

var siteLayoutTheme1FragmentFiles = map[string]string{
	"header":         "header.html",
	"footer":         "footer.html",
	"home_body":      "home_body.html",
	"article_body":   "article_body.html",
	"sidebar":        "sidebar.html",
	"rss":            "rss.html",
	"author_profile": "author_profile.html",
	"tag":            "tag.html",
	"kategori":       "kategori.html",
	"search_page":    "search_page.html",
}

var siteLayoutTheme2FragmentFiles = map[string]string{
	"header":         "header.html",
	"footer":         "footer.html",
	"home_body":      "home_body.html",
	"article_body":   "article_body.html",
	"sidebar":        "sidebar.html",
	"rss":            "rss.html",
	"author_profile": "author_profile.html",
	"tag":            "tag.html",
	"kategori":       "kategori.html",
	"search_page":    "search_page.html",
}

var siteLayoutTheme3FragmentFiles = map[string]string{
	"header":         "header.html",
	"footer":         "footer.html",
	"home_body":      "home_body.html",
	"article_body":   "article_body.html",
	"sidebar":        "sidebar.html",
	"rss":            "rss.html",
	"author_profile": "author_profile.html",
	"tag":            "tag.html",
	"kategori":       "kategori.html",
	"search_page":    "search_page.html",
}

func seedSiteLayout(db *gorm.DB) error {
	defaultDir := filepath.Join(findProjectRoot(), "db", "seeds", "layouts", "default")
	defaultFrags, err := loadSiteLayoutFragmentsFromMap(defaultDir, siteLayoutDefaultFragmentFiles)
	if err != nil {
		return err
	}
	theme1Dir := filepath.Join(findProjectRoot(), "db", "seeds", "layouts", "theme1")
	theme1Frags, err := loadSiteLayoutFragmentsFromMap(theme1Dir, siteLayoutTheme1FragmentFiles)
	if err != nil {
		return err
	}
	theme2Dir := filepath.Join(findProjectRoot(), "db", "seeds", "layouts", "theme2")
	theme2Frags, err := loadSiteLayoutFragmentsFromMap(theme2Dir, siteLayoutTheme2FragmentFiles)
	if err != nil {
		return err
	}
	theme3Dir := filepath.Join(findProjectRoot(), "db", "seeds", "layouts", "theme3")
	theme3Frags, err := loadSiteLayoutFragmentsFromMap(theme3Dir, siteLayoutTheme3FragmentFiles)
	if err != nil {
		return err
	}

	defaults := []entity.SiteLayout{
		siteLayoutFromFragments("default", defaultFrags),
		siteLayoutFromFragments("theme1", theme1Frags),
		siteLayoutFromFragments("theme2", theme2Frags),
		siteLayoutFromFragments("theme3", theme3Frags),
	}

	for _, def := range defaults {
		var existing entity.SiteLayout
		if err := db.Where("layout_name = ?", def.LayoutName).First(&existing).Error; err == nil {
			layout := def
			layout.ID = existing.ID
			layout.IsUsed = existing.IsUsed
			// MySQL may store NULL created_at (e.g. manual INSERT); never write zero-date.
			if existing.CreatedAt.IsZero() {
				layout.CreatedAt = time.Now().UTC()
			} else {
				layout.CreatedAt = existing.CreatedAt
			}
			layout.UpdatedAt = time.Now().UTC()
			if err := db.Save(&layout).Error; err != nil {
				return err
			}
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		layout := def
		now := time.Now().UTC()
		layout.CreatedAt = now
		layout.UpdatedAt = now
		if err := db.Create(&layout).Error; err != nil {
			return err
		}
	}

	// Preserve the site's chosen custom layout on disk (is_used). Never auto-activate a seed theme.
	return reapplyUsedSiteLayoutToDisk(db)
}

// ReapplyUsedLayout writes the active (is_used) layout fragments to public/templates.
// Call after git checkout/deploy so live files match the site's chosen layout without Admin Apply.
func ReapplyUsedLayout(db *gorm.DB) error {
	return reapplyUsedSiteLayoutToDisk(db)
}

func siteLayoutFragmentContent(row *entity.SiteLayout, fragment string) string {
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

func reapplyUsedSiteLayoutToDisk(db *gorm.DB) error {
	var used entity.SiteLayout
	err := db.Where("is_used = ?", true).Order("id ASC").First(&used).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	root := findProjectRoot()
	applied := 0
	var skippedMissing []string
	for _, fragment := range siteLayoutLiveFragmentOrder {
		content := strings.ReplaceAll(strings.TrimSpace(siteLayoutFragmentContent(&used, fragment)), "\r\n", "\n")
		if content == "" {
			continue
		}
		rel, ok := siteLayoutLiveFragmentFiles[fragment]
		if !ok {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		if _, statErr := os.Stat(path); statErr != nil {
			if os.IsNotExist(statErr) {
				skippedMissing = append(skippedMissing, path)
				continue
			}
			return statErr
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if _, parseErr := template.New("seed-apply-" + fragment).Funcs(clienttpl.TemplateFuncs()).Parse(content); parseErr != nil {
			return fmt.Errorf("reapply used layout %q fragment %q: %w", used.LayoutName, fragment, parseErr)
		}
		if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
			return fmt.Errorf("reapply used layout %q fragment %q: %w", used.LayoutName, fragment, writeErr)
		}
		applied++
	}
	if applied == 0 {
		if len(skippedMissing) > 0 {
			log.Printf("site layout seed: public pages use the portfolio template; skipped %d missing client fragments", len(skippedMissing))
			return nil
		}
		return fmt.Errorf("reapply used layout %q: no non-empty fragments to write", used.LayoutName)
	}
	clienttpl.InvalidateClientTemplateCache()
	log.Printf("site layout seed: re-applied active layout %q to %d template files", used.LayoutName, applied)
	return nil
}

func siteLayoutFromFragments(name string, frags map[string]string) entity.SiteLayout {
	return entity.SiteLayout{
		LayoutName:    name,
		Header:        frags["header"],
		Footer:        frags["footer"],
		HomeBody:      frags["home_body"],
		ArticleBody:   frags["article_body"],
		Sidebar:       frags["sidebar"],
		RSS:           frags["rss"],
		AuthorProfile: frags["author_profile"],
		Tag:           frags["tag"],
		Kategori:      frags["kategori"],
		SearchPage:    frags["search_page"],
		Misc:          "",
		LatestPrompt:  siteLayoutAIPrompt,
		CurrentPrompt: siteLayoutAIPrompt,
		ImageRef:      "",
		CSSData:       "",
		JSData:        "",
		IsUsed:        false,
	}
}

func loadSiteLayoutFragmentsFromMap(baseDir string, files map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(files))
	for key, rel := range files {
		path := rel
		if !filepath.IsAbs(rel) {
			path = filepath.Join(baseDir, rel)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out[key] = strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	return out, nil
}

func findProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		if looksLikeProjectRoot(wd) {
			return wd
		}
		// Deploy Ubuntu: WorkingDirectory=/opt/<slug> with repo/ + public → repo/public
		if repo := filepath.Join(wd, "repo"); looksLikeProjectRoot(repo) {
			return repo
		}
		dir := wd
		for i := 0; i < 8; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			if looksLikeProjectRoot(dir) {
				return dir
			}
			if repo := filepath.Join(dir, "repo"); looksLikeProjectRoot(repo) {
				return repo
			}
		}
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(file) // db/seeds
		root := filepath.Clean(filepath.Join(dir, "..", ".."))
		if looksLikeProjectRoot(root) {
			return root
		}
	}
	return "."
}

func looksLikeProjectRoot(dir string) bool {
	if dir == "" {
		return false
	}
	_, err1 := os.Stat(filepath.Join(dir, "go.mod"))
	_, err2 := os.Stat(filepath.Join(dir, "public", "templates"))
	return err1 == nil && err2 == nil
}
