package handler

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	appconfig "github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
)

var nonSlugCharRe = regexp.MustCompile(`[^a-z0-9\s-]`)
var multiDashRe = regexp.MustCompile(`-{2,}`)
var wpBlockRe = regexp.MustCompile(`<!-- wp:.*?-->`)
var wpBlockEndRe = regexp.MustCompile(`<!-- /wp:.*?-->`)
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)
var wpShortcodeRe = regexp.MustCompile(`\[[^\]]+\]`)

// Yoast SEO meta keys
const (
	yoastMetaDesc  = "_yoast_wpseo_metadesc"
	yoastFocusKW   = "_yoast_wpseo_focuskw"
	yoastMetaKW    = "_yoast_wpseo_metakeywords"
	yoastTitle     = "_yoast_wpseo_title"
	yoastOpengraph = "_yoast_wpseo_opengraph-image"
)

// WpImportResult holds the outcome of a WordPress import.
type WpImportResult struct {
	AuthorsCreated    int `json:"authors_created"`
	AuthorsSkipped    int `json:"authors_skipped"`
	CategoriesCreated int `json:"categories_created"`
	CategoriesSkipped int `json:"categories_skipped"`
	TagsCreated       int `json:"tags_created"`
	TagsSkipped       int `json:"tags_skipped"`
	PostsCreated      int `json:"posts_created"`
	PostsSkipped      int `json:"posts_skipped"`
	PagesCreated      int `json:"pages_created"`
	PagesSkipped      int `json:"pages_skipped"`
	LinksCreated      int `json:"post_tag_links_created"`
	ImagesDownloaded  int `json:"images_downloaded"`
	ImagesFailed      int `json:"images_failed"`
}

// WpPreviewResult holds preview counts before import.
type WpPreviewResult struct {
	Authors    int `json:"authors"`
	Categories int `json:"categories"`
	Tags       int `json:"tags"`
	Posts      int `json:"posts"`
	Pages      int `json:"pages"`
}

func wpSlugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "&", "and")
	s = nonSlugCharRe.ReplaceAllString(s, "-")
	s = multiDashRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("item-%d", time.Now().UnixNano()%100000)
	}
	return s
}

func wpEnsureUniqueSlug(db *gorm.DB, table, base string, excludeID uint) string {
	slug := base
	for i := 0; i < 1000; i++ {
		candidate := slug
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		var n int64
		q := db.Table(table).Where("slug = ?", candidate)
		if excludeID > 0 {
			q = q.Where("id <> ?", excludeID)
		}
		q.Count(&n)
		if n == 0 {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano()%100000)
}

func wpMapStatus(wpStatus string) string {
	switch strings.ToLower(wpStatus) {
	case "publish":
		return "published"
	case "draft":
		return "draft"
	case "private", "pending":
		return "draft"
	case "trash", "future":
		return "draft"
	default:
		return "draft"
	}
}

func wpCleanContent(content string) string {
	// Remove Gutenberg block comments but keep inner content
	content = wpBlockRe.ReplaceAllString(content, "")
	content = wpBlockEndRe.ReplaceAllString(content, "")
	// Remove WordPress shortcodes
	content = wpShortcodeRe.ReplaceAllString(content, "")
	// Clean up excessive whitespace
	lines := strings.Split(content, "\n")
	var cleaned []string
	prevEmpty := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevEmpty {
				cleaned = append(cleaned, "")
			}
			prevEmpty = true
			continue
		}
		cleaned = append(cleaned, line)
		prevEmpty = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func wpTruncate(s string, maxLen int) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Strip HTML tags for meta fields
	s = htmlTagRe.ReplaceAllString(s, "")
	runes := []rune(s)
	if len(runes) > maxLen {
		// Find last period within range to avoid cutting words
		cropped := string(runes[:maxLen])
		if idx := strings.LastIndex(cropped, ". "); idx > maxLen/2 {
			cropped = cropped[:idx+1]
		} else if idx := strings.LastIndex(cropped, "."); idx > maxLen/2 {
			cropped = cropped[:idx+1]
		} else if idx := strings.LastIndex(cropped, " "); idx > 0 {
			cropped = cropped[:idx]
		}
		result := strings.TrimSpace(cropped)
		if result != "" {
			return &result
		}
		// Hard truncation fallback
		runes = runes[:maxLen]
	}
	result := strings.TrimSpace(string(runes))
	if result == "" {
		return nil
	}
	return &result
}

func wpRandomPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%"
	b := make([]byte, 16)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func wpHashPassword(plain string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(hash)
}

// buildWpPostMetaMap builds a map of postID -> metaKey -> metaValue for quick lookup.
func buildWpPostMetaMap(metaRows []WpPostMeta) map[int]map[string]string {
	result := make(map[int]map[string]string)
	for _, m := range metaRows {
		if _, ok := result[m.PostID]; !ok {
			result[m.PostID] = make(map[string]string)
		}
		result[m.PostID][m.MetaKey] = m.MetaValue
	}
	return result
}

// buildWpUserMetaMap builds a map of userID -> metaKey -> metaValue.
func buildWpUserMetaMap(metaRows []WpUserMeta) map[int]map[string]string {
	result := make(map[int]map[string]string)
	for _, m := range metaRows {
		if _, ok := result[m.UserID]; !ok {
			result[m.UserID] = make(map[string]string)
		}
		result[m.UserID][m.MetaKey] = m.MetaValue
	}
	return result
}

// buildWpTermTaxonomyMap builds a map of termTaxonomyID -> WpTermTaxonomy.
func buildWpTermTaxonomyMap(rows []WpTermTaxonomy) map[int]WpTermTaxonomy {
	result := make(map[int]WpTermTaxonomy)
	for _, r := range rows {
		result[r.TermTaxonomyID] = r
	}
	return result
}

// buildWpTermMetaMap builds a map of termID -> metaKey -> metaValue.
func buildWpTermMetaMap(metaRows []WpTermMeta) map[int]map[string]string {
	result := make(map[int]map[string]string)
	for _, m := range metaRows {
		if _, ok := result[m.TermID]; !ok {
			result[m.TermID] = make(map[string]string)
		}
		result[m.TermID][m.MetaKey] = m.MetaValue
	}
	return result
}

// buildWpTermMap builds a map of termID -> WpTerm.
func buildWpTermMap(rows []WpTerm) map[int]WpTerm {
	result := make(map[int]WpTerm)
	for _, t := range rows {
		result[t.TermID] = t
	}
	return result
}

// buildPostTermRels builds a map of postID -> []termTaxonomyID.
func buildPostTermRels(rels []WpTermRelationship) map[int][]int {
	result := make(map[int][]int)
	for _, r := range rels {
		result[r.ObjectID] = append(result[r.ObjectID], r.TermTaxonomyID)
	}
	return result
}

// resolveDisplayName gets display_name from usermeta or falls back to user_login.
func resolveDisplayName(userID int, userMeta map[int]map[string]string) string {
	if meta, ok := userMeta[userID]; ok {
		if dn, ok := meta["display_name"]; ok && strings.TrimSpace(dn) != "" {
			return strings.TrimSpace(dn)
		}
	}
	return ""
}

func parseWPTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" || s == "0000-00-00 00:00:00" || s == "0000-00-00" || strings.HasPrefix(s, "0000-00-00") {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// mapWPUserToEntity maps a WordPress user to a bebii-blog user.
// Password is randomized; the user must reset it.
func mapWPUserToEntity(wpUser WpUser, userMeta map[int]map[string]string) entity.User {
	displayName := resolveDisplayName(wpUser.ID, userMeta)
	if displayName == "" {
		displayName = wpUser.Login
	}
	randomPass := wpRandomPassword()
	u := entity.User{
		Name:     displayName,
		Username: strings.ToLower(wpUser.Login),
		Email:    strings.ToLower(strings.TrimSpace(wpUser.Email)),
		Password: wpHashPassword(randomPass),
		Role:     "writer",
		Enabled:  true,
	}
	regTime := parseWPTime(wpUser.UserRegistered)
	if !regTime.IsZero() {
		u.CreatedAt = regTime
		u.UpdatedAt = regTime
	}
	return u
}

// mapWPCategoryToEntity maps a WordPress category term to a bebii-blog category.
func mapWPCategoryToEntity(term WpTerm, taxonomy WpTermTaxonomy, yoastMeta map[string]string) entity.Category {
	title := ""
	var metaDesc *string
	var metaKeywords *string
	if yoastMeta != nil {
		if v, ok := yoastMeta[yoastTitle]; ok {
			title = strings.TrimSpace(strings.Trim(v, "\""))
		}
		if v, ok := yoastMeta[yoastMetaDesc]; ok {
			metaDesc = wpTruncate(v, 500)
		}
		if v, ok := yoastMeta[yoastMetaKW]; ok {
			metaKeywords = wpTruncate(v, 500)
		}
		// Fallback to generic meta_description field in termmeta
		if metaDesc == nil {
			if v, ok := yoastMeta["meta_description"]; ok {
				metaDesc = wpTruncate(v, 500)
			}
		}
		// Fallback to generic meta_keywords field in termmeta
		if metaKeywords == nil {
			if v, ok := yoastMeta["meta_keywords"]; ok {
				metaKeywords = wpTruncate(v, 500)
			}
		}
	}
	// Final fallback: use category name as keyword
	if metaKeywords == nil {
		if name := strings.TrimSpace(term.Name); name != "" {
			metaKeywords = &name
		}
	}
	description := ""
	if metaDesc != nil {
		description = *metaDesc
	}
	keyword := ""
	if metaKeywords != nil {
		keyword = *metaKeywords
	}
	return entity.Category{
		Name:        term.Name,
		Slug:        term.Slug,
		Title:       title,
		Keyword:     keyword,
		Description: description,
		Sort:        1,
		Enabled:     false,
	}
}

// mapWPTagToEntity maps a WordPress tag term to a bebii-blog tag.
func mapWPTagToEntity(term WpTerm) entity.Tag {
	return entity.Tag{
		Name: term.Name,
		Slug: term.Slug,
	}
}

// mapWPPostToEntity maps a WordPress post to a bebii-blog post.
func mapWPPostToEntity(
	wpPost WpPost,
	postMeta map[string]string,
	categoryID uint,
	tagIDs []uint,
	authorMap map[int]uint,
) entity.Post {
	status := wpMapStatus(wpPost.PostStatus)
	content := wpCleanContent(wpPost.PostContent)

	var metaDesc *string
	var metaKeywords *string

	// Extract Yoast SEO meta
	if postMeta != nil {
		if v, ok := postMeta[yoastMetaDesc]; ok {
			metaDesc = wpTruncate(v, 500)
		}
		if v, ok := postMeta[yoastFocusKW]; ok {
			metaKeywords = wpTruncate(v, 500)
		}
		// Fallback: try generic meta keys
		if metaDesc == nil {
			if v, ok := postMeta["meta_description"]; ok {
				metaDesc = wpTruncate(v, 500)
			}
		}
		if metaKeywords == nil {
			if v, ok := postMeta["meta_keywords"]; ok {
				metaKeywords = wpTruncate(v, 500)
			}
		}
	}

	// Featured image is resolved and downloaded in ImportWordPressData when downloadImages is enabled.
	// When disabled, we don't set a placeholder — the image field stays nil.

	// Resolve author
	authorID := uint(0)
	if id, ok := authorMap[wpPost.PostAuthor]; ok {
		authorID = id
	}

	createdTime := parseWPTime(wpPost.PostDate)
	modifiedTime := parseWPTime(wpPost.PostModified)

	if createdTime.IsZero() && !modifiedTime.IsZero() {
		createdTime = modifiedTime
	}
	if modifiedTime.IsZero() && !createdTime.IsZero() {
		modifiedTime = createdTime
	}

	p := entity.Post{
		Title:          wpPost.PostTitle,
		Slug:           wpSlugify(wpPost.PostName),
		Content:        content,
		ImageOrigin:    entity.ImageOriginManual,
		MetaDesc:       metaDesc,
		MetaKeywords:   metaKeywords,
		IncludeSource:  false,
		Status:         status,
		AuthorID:       authorID,
		CategoryID:     categoryID,
	}
	if !createdTime.IsZero() {
		p.CreatedAt = createdTime
	}
	if !modifiedTime.IsZero() {
		p.UpdatedAt = modifiedTime
	}
	return p
}

// mapWPPageToEntity maps a WordPress page to a bebii-blog page.
func mapWPPageToEntity(wpPost WpPost) entity.Page {
	status := wpMapStatus(wpPost.PostStatus)
	content := wpCleanContent(wpPost.PostContent)

	createdTime := parseWPTime(wpPost.PostDate)
	modifiedTime := parseWPTime(wpPost.PostModified)

	if createdTime.IsZero() && !modifiedTime.IsZero() {
		createdTime = modifiedTime
	}
	if modifiedTime.IsZero() && !createdTime.IsZero() {
		modifiedTime = createdTime
	}

	page := entity.Page{
		Title:   wpPost.PostTitle,
		Slug:    wpSlugify(wpPost.PostName),
		Content: content,
		Status:  status,
	}
	if !createdTime.IsZero() {
		page.CreatedAt = createdTime
	}
	if !modifiedTime.IsZero() {
		page.UpdatedAt = modifiedTime
	}
	return page
}

// isNonASCIIWord checks if a string contains non-ASCII characters (e.g., CJK).
func isNonASCIIWord(s string) bool {
	for _, r := range s {
		if r > 127 && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// wpFallbackSlug generates a slug for titles that can't be slugified (e.g., all non-ASCII).
func wpFallbackSlug(title string) string {
	slug := wpSlugify(title)
	if slug == "" || slug == "-" {
		// Use a hash-like approach from title length + runes
		sum := 0
		for _, r := range title {
			sum += int(r)
		}
		slug = fmt.Sprintf("post-%d", sum%100000)
	}
	return slug
}

// ImportWordPressData performs the actual import into the database.
func ImportWordPressData(db *gorm.DB, data *WpParsedData, downloadImages bool, cfg appconfig.Config) (*WpImportResult, error) {
	result := &WpImportResult{}

	// Build lookup maps
	wpUserMetaMap := buildWpUserMetaMap(data.UserMeta)
	wpTermMap := buildWpTermMap(data.Terms)
	wpTermTaxMap := buildWpTermTaxonomyMap(data.TermTaxonomies)
	wpPostMetaMap := buildWpPostMetaMap(data.PostMeta)
	wpTermMetaMap := buildWpTermMetaMap(data.TermMeta)
	postTermRels := buildPostTermRels(data.TermRelationships)

	// Build attachment map for featured image resolution
	attachmentMap := buildAttachmentMap(data.Posts)

	// Prepare image store if download is enabled
	var imgStore storage.ObjectStore
	var baseDomain string
	if downloadImages {
		ctx := context.Background()
		imgStore = storage.PostAssetStore(ctx, db, cfg)
		// Try to extract base domain from first post GUID
		for _, p := range data.Posts {
			if strings.TrimSpace(p.Guid) != "" {
				if u, err := (func() (*url.URL, error) {
					return url.Parse(p.Guid)
				})(); err == nil && u != nil {
					baseDomain = u.Scheme + "://" + u.Host
				}
				break
			}
		}
	}

	// Separate terms by taxonomy
	categoryTermIDs := make(map[int]bool)   // termID -> true for categories
	termTaxByTermID := make(map[int]WpTermTaxonomy) // termID -> taxonomy row
	for _, tax := range data.TermTaxonomies {
		if strings.ToLower(tax.Taxonomy) == "category" {
			categoryTermIDs[tax.TermID] = true
		}
		termTaxByTermID[tax.TermID] = tax
	}

	// 1. Import authors
	authorMap := make(map[int]uint) // wpUserID -> bebiiUserID
	for _, wpUser := range data.Users {
		var existing entity.User
		err := db.Where("LOWER(email) = ?", strings.ToLower(wpUser.Email)).First(&existing).Error
		if err == nil && existing.ID > 0 {
			authorMap[wpUser.ID] = existing.ID
			result.AuthorsSkipped++
			continue
		}
		user := mapWPUserToEntity(wpUser, wpUserMetaMap)
		if err := db.Create(&user).Error; err != nil {
			result.AuthorsSkipped++
			continue
		}
		authorMap[wpUser.ID] = user.ID
		result.AuthorsCreated++
	}

	// Fallback: use first available author
	var fallbackAuthorID uint
	var firstAuthor entity.User
	if err := db.Where("role IN ? AND enabled = ?", []string{"admin", "writer"}, true).Order("id asc").First(&firstAuthor).Error; err == nil {
		fallbackAuthorID = firstAuthor.ID
	}

	// 2. Import categories (root first, then children)
	type catImport struct {
		term     WpTerm
		tax      WpTermTaxonomy
		parentWP int // WP term parent
	}
	var rootCats []catImport
	var childCats []catImport
	for _, tax := range data.TermTaxonomies {
		if strings.ToLower(tax.Taxonomy) != "category" {
			continue
		}
		term, ok := wpTermMap[tax.TermID]
		if !ok {
			continue
		}
		if tax.Parent == 0 {
			rootCats = append(rootCats, catImport{term: term, tax: tax, parentWP: tax.Parent})
		} else {
			childCats = append(childCats, catImport{term: term, tax: tax, parentWP: tax.Parent})
		}
	}

	wpCategoryIDToBebii := make(map[int]uint) // wpTermID -> bebiiCategoryID
	sortCounter := 0

	// Create root categories
	for _, ci := range rootCats {
		yoastMeta := wpTermMetaMap[ci.term.TermID]
		cat := mapWPCategoryToEntity(ci.term, ci.tax, yoastMeta)
		cat.Slug = wpEnsureUniqueSlug(db, "categories", cat.Slug, 0)
		sortCounter++
		cat.Sort = sortCounter
		if err := db.Create(&cat).Error; err != nil {
			result.CategoriesSkipped++
			continue
		}
		wpCategoryIDToBebii[ci.term.TermID] = cat.ID
		result.CategoriesCreated++
	}

	// Create child categories (max depth 1 per bebii-blog constraint)
	for _, ci := range childCats {
		yoastMeta := wpTermMetaMap[ci.term.TermID]
		cat := mapWPCategoryToEntity(ci.term, ci.tax, yoastMeta)
		cat.Slug = wpEnsureUniqueSlug(db, "categories", cat.Slug, 0)
		sortCounter++
		cat.Sort = sortCounter
		if parentBebiiID, ok := wpCategoryIDToBebii[ci.parentWP]; ok {
			cat.ParentID = &parentBebiiID
		}
		if err := db.Create(&cat).Error; err != nil {
			result.CategoriesSkipped++
			continue
		}
		wpCategoryIDToBebii[ci.term.TermID] = cat.ID
		result.CategoriesCreated++
	}

	// Fallback category
	var fallbackCategoryID uint
	var firstCat entity.Category
	if err := db.Order("id asc").First(&firstCat).Error; err == nil {
		fallbackCategoryID = firstCat.ID
	}

	// 3. Import tags
	wpTagIDToBebii := make(map[int]uint) // wpTermID -> bebiiTagID
	for _, tax := range data.TermTaxonomies {
		if strings.ToLower(tax.Taxonomy) != "post_tag" {
			continue
		}
		term, ok := wpTermMap[tax.TermID]
		if !ok {
			continue
		}
		tag := mapWPTagToEntity(term)
		tag.Slug = wpEnsureUniqueSlug(db, "tags", tag.Slug, 0)
		if err := db.Create(&tag).Error; err != nil {
			result.TagsSkipped++
			continue
		}
		wpTagIDToBebii[term.TermID] = tag.ID
		result.TagsCreated++
	}

	// 4. Import pages
	for _, wpPost := range data.Posts {
		if wpPost.PostType != "page" {
			continue
		}
		page := mapWPPageToEntity(wpPost)
		if strings.TrimSpace(page.Title) == "" {
			result.PagesSkipped++
			continue
		}
		if page.Slug == "" || page.Slug == "-" {
			page.Slug = wpFallbackSlug(page.Title)
		}
		page.Slug = wpEnsureUniqueSlug(db, "pages", page.Slug, 0)

		// Download content images if enabled
		if downloadImages && imgStore != nil && strings.TrimSpace(page.Content) != "" {
			ctx := context.Background()
			rewritten, imgCount := rewriteContentImages(ctx, page.Content, imgStore, baseDomain)
			if imgCount > 0 {
				page.Content = rewritten
				result.ImagesDownloaded += imgCount
			}
		}

		if err := db.Create(&page).Error; err != nil {
			result.PagesSkipped++
			continue
		}
		result.PagesCreated++
	}

	// 5. Import posts
	postIdx := 0
	for _, wpPost := range data.Posts {
		if wpPost.PostType != "post" {
			continue
		}
		if strings.TrimSpace(wpPost.PostTitle) == "" {
			result.PostsSkipped++
			continue
		}
		postIdx++

		// Resolve category from term relationships
		categoryID := fallbackCategoryID
		if termTaxIDs, ok := postTermRels[wpPost.ID]; ok {
			for _, ttID := range termTaxIDs {
				if tax, ok := wpTermTaxMap[ttID]; ok && strings.ToLower(tax.Taxonomy) == "category" {
					if bid, ok := wpCategoryIDToBebii[tax.TermID]; ok {
						categoryID = bid
						break
					}
				}
			}
		}

		// Resolve tags from term relationships
		var tagIDs []uint
		if termTaxIDs, ok := postTermRels[wpPost.ID]; ok {
			for _, ttID := range termTaxIDs {
				if tax, ok := wpTermTaxMap[ttID]; ok && strings.ToLower(tax.Taxonomy) == "post_tag" {
					if bid, ok := wpTagIDToBebii[tax.TermID]; ok {
						tagIDs = append(tagIDs, bid)
					}
				}
			}
		}

		// Get post meta
		postMeta := wpPostMetaMap[wpPost.ID]

		// Resolve author
		authorID := fallbackAuthorID
		if id, ok := authorMap[wpPost.PostAuthor]; ok && id > 0 {
			authorID = id
		}

		post := mapWPPostToEntity(wpPost, postMeta, categoryID, tagIDs, authorMap)
		if post.Slug == "" || post.Slug == "-" {
			post.Slug = wpFallbackSlug(post.Title)
		}
		post.Slug = wpEnsureUniqueSlug(db, "posts", post.Slug, 0)
		post.AuthorID = authorID

		// Download featured image if enabled
		if downloadImages && imgStore != nil && postMeta != nil {
			if thumbID, ok := postMeta["_thumbnail_id"]; ok {
				thumbURL := resolveAttachmentURL(thumbID, attachmentMap)
				if thumbURL != "" {
					ctx := context.Background()
					savedURL := downloadWPImage(ctx, thumbURL, imgStore, post.Slug)
					if savedURL != "" {
						post.Image = &savedURL
						post.ImageOrigin = entity.ImageOriginManual
						result.ImagesDownloaded++
					} else {
						result.ImagesFailed++
					}
				}
			}
		}

		// Download content images if enabled
		if downloadImages && imgStore != nil && strings.TrimSpace(post.Content) != "" {
			ctx := context.Background()
			rewritten, imgCount := rewriteContentImages(ctx, post.Content, imgStore, baseDomain)
			if imgCount > 0 {
				post.Content = rewritten
				result.ImagesDownloaded += imgCount
			}
		}

		if err := db.Create(&post).Error; err != nil {
			result.PostsSkipped++
			continue
		}
		result.PostsCreated++

		// Link tags
		if len(tagIDs) > 0 {
			for _, tagID := range tagIDs {
				db.Exec("INSERT IGNORE INTO post_tags (post_id, tag_id) VALUES (?, ?)", post.ID, tagID)
				result.LinksCreated++
			}
		}
	}
	return result, nil
}
