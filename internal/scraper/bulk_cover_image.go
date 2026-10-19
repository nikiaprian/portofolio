package scraper

import (
	"context"
	"encoding/xml"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/storage"
	"gorm.io/gorm"
)

const (
	bulkCoverMaxFeeds           = 5
	bulkCoverMaxItemsPerFeed    = 80
	bulkCoverTopCandidates      = 8
	bulkCoverMinMatchScore      = 0.26
	bulkCoverArticleFetchBudget = 28
)

type bulkScoredItem struct {
	item  rssItem
	link  string
	score float64
}

// BulkCoverImageFromCategoryFeeds tries to find a hero image for a bulk-generated post by matching
// the title against items from the category's enabled scraper feed URLs (Admin → Scraper settings).
// Returns nil when no feeds, no good title match, or image download fails.
func BulkCoverImageFromCategoryFeeds(ctx context.Context, db *gorm.DB, store storage.ObjectStore, categoryID uint, bulkTitle, postSlug string) *string {
	if db == nil || categoryID == 0 {
		return nil
	}
	bulkTitle = strings.TrimSpace(bulkTitle)
	postSlug = strings.TrimSpace(postSlug)
	if bulkTitle == "" || postSlug == "" {
		return nil
	}

	var cat entity.Category
	if err := db.First(&cat, categoryID).Error; err != nil {
		return nil
	}
	slug := strings.TrimSpace(cat.Slug)
	if slug == "" {
		return nil
	}

	var row entity.ScraperCategoryFeed
	if err := db.Where("LOWER(TRIM(category_slug)) = ? AND enabled = ?", entity.CanonicalCategorySlug(slug), true).First(&row).Error; err != nil {
		return nil
	}
	feeds := splitScraperFeedURLs(row.FeedURLs)
	if len(feeds) == 0 {
		return nil
	}
	if len(feeds) > bulkCoverMaxFeeds {
		feeds = feeds[:bulkCoverMaxFeeds]
	}

	client := &http.Client{Timeout: 28 * time.Second}
	storeMaxEdge, storeWebPQ := repositories.PostImageStoreLimitsFromDB(db)
	var pool []bulkScoredItem
	var loosePool []bulkScoredItem
	for _, rawFeedURL := range feeds {
		if err := ctx.Err(); err != nil {
			break
		}
		body, err := fetchFeedBytes(ctx, client, rawFeedURL, 0)
		if err != nil {
			continue
		}
		safe := sanitizeXMLAmpersandsPreservingCDATA(string(body))
		var doc rssDocument
		dec := xml.NewDecoder(strings.NewReader(safe))
		dec.Strict = false
		if err := dec.Decode(&doc); err != nil {
			continue
		}
		n := 0
		for _, it := range doc.Channel.Items {
			if n >= bulkCoverMaxItemsPerFeed {
				break
			}
			t := strings.TrimSpace(it.Title)
			if t == "" {
				continue
			}
			link := rssItemLink(it)
			if link == "" {
				continue
			}
			sc := titleMatchScoreForBulk(bulkTitle, t)
			if sc < bulkCoverMinMatchScore {
				loosePool = append(loosePool, bulkScoredItem{item: it, link: link, score: sc})
				continue
			}
			pool = append(pool, bulkScoredItem{item: it, link: link, score: sc})
			n++
		}
		n = 0
		for _, entry := range doc.Entries {
			if n >= bulkCoverMaxItemsPerFeed {
				break
			}
			t := strings.TrimSpace(entry.Title)
			if t == "" {
				continue
			}
			link := atomEntryLink(entry)
			if link == "" {
				continue
			}
			raw := firstNonEmpty(entry.Content, entry.Summary)
			synth := rssItem{
				Title:          entry.Title,
				Link:           link,
				Description:    raw,
				ContentEncoded: raw,
			}
			sc := titleMatchScoreForBulk(bulkTitle, t)
			if sc < bulkCoverMinMatchScore {
				loosePool = append(loosePool, bulkScoredItem{item: synth, link: link, score: sc})
				continue
			}
			pool = append(pool, bulkScoredItem{item: synth, link: link, score: sc})
			n++
		}
	}
	if len(pool) == 0 {
		// Fallback mode: title matching may fail for paraphrased AI titles; still try
		// recent feed items in crawl order so bulk posts can get a cover image.
		pool = loosePool
		if len(pool) == 0 {
			return nil
		}
	}

	if len(loosePool) != len(pool) {
		sort.SliceStable(pool, func(i, j int) bool {
			if pool[i].score != pool[j].score {
				return pool[i].score > pool[j].score
			}
			return pool[i].link < pool[j].link
		})
	}

	seenLink := make(map[string]struct{}, len(pool))
	ordered := make([]bulkScoredItem, 0, bulkCoverTopCandidates)
	for _, it := range pool {
		k := strings.ToLower(strings.TrimSpace(it.link))
		if k == "" {
			continue
		}
		if _, dup := seenLink[k]; dup {
			continue
		}
		seenLink[k] = struct{}{}
		ordered = append(ordered, it)
		if len(ordered) >= bulkCoverTopCandidates {
			break
		}
	}

	fetchesLeft := bulkCoverArticleFetchBudget
	for _, cand := range ordered {
		if err := ctx.Err(); err != nil {
			break
		}
		raw, hero, _, left := rssItemBodyHTML(ctx, client, cand.item, cand.link, fetchesLeft)
		fetchesLeft = left
		in := postCandidate{
			Title:        strings.TrimSpace(cand.item.Title),
			Link:         cand.link,
			Description:  raw,
			HeroImageURL: strings.TrimSpace(hero),
		}
		if img := resolveScrapedPostImage(ctx, client, store, in, cand.link, postSlug, storeMaxEdge, storeWebPQ); img != nil && strings.TrimSpace(*img) != "" {
			return img
		}
	}
	return nil
}

func titleMatchScoreForBulk(bulk, item string) float64 {
	bulk = strings.TrimSpace(strings.ToLower(html.UnescapeString(bulk)))
	item = strings.TrimSpace(strings.ToLower(html.UnescapeString(item)))
	if bulk == "" || item == "" {
		return 0
	}
	if bulk == item {
		return 1
	}
	bKey := strings.TrimSpace(spaceSeqRegex.ReplaceAllString(nonSlugRegex.ReplaceAllString(bulk, " "), " "))
	iKey := strings.TrimSpace(spaceSeqRegex.ReplaceAllString(nonSlugRegex.ReplaceAllString(item, " "), " "))
	if bKey == "" || iKey == "" {
		return 0
	}
	if bKey == iKey {
		return 0.97
	}
	if utf8.RuneCountInString(bKey) >= 12 {
		if strings.Contains(iKey, bKey) {
			return 0.9
		}
	}
	if utf8.RuneCountInString(iKey) >= 12 {
		if strings.Contains(bKey, iKey) {
			return 0.9
		}
	}
	j := jaccardTokenSets(bKey, iKey, 3)
	if j >= 0.5 {
		return 0.45 + 0.5*j
	}
	if j >= bulkCoverMinMatchScore {
		return j
	}
	return 0
}

func jaccardTokenSets(a, b string, minRunes int) float64 {
	setA := tokenSetMinRunes(a, minRunes)
	setB := tokenSetMinRunes(b, minRunes)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for tok := range setA {
		if _, ok := setB[tok]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenSetMinRunes(s string, minRunes int) map[string]struct{} {
	out := make(map[string]struct{})
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, `"'“”`)
		if utf8.RuneCountInString(w) < minRunes {
			continue
		}
		out[w] = struct{}{}
	}
	return out
}
