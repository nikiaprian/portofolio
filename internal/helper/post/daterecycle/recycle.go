package daterecycle

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

const ua = "BebiiBlog-PostDateUrlRecycle/1.0"

var (
	qMu     sync.Mutex
	queue   []string
	qSeen   = make(map[string]struct{})
	qLastAt time.Time
)

// SplitURLs splits newline-separated (and trimmed) URLs; empty lines skipped.
func SplitURLs(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// EnqueueURLs appends unique URLs to the in-memory FIFO queue.
// Returns (added, totalAfter): how many were actually appended (after dedupe) and the new queue length.
func EnqueueURLs(urls []string) (added int, totalAfter int) {
	qMu.Lock()
	defer qMu.Unlock()
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, dup := qSeen[u]; dup {
			continue
		}
		qSeen[u] = struct{}{}
		queue = append(queue, u)
		added++
	}
	return added, len(queue)
}

// QueueLen returns the current number of URLs waiting to be processed.
func QueueLen() int {
	qMu.Lock()
	defer qMu.Unlock()
	return len(queue)
}

// ResetQueue clears the queue and the dedupe set; also resets the gap timer.
func ResetQueue() {
	qMu.Lock()
	defer qMu.Unlock()
	queue = nil
	qSeen = make(map[string]struct{})
	qLastAt = time.Time{}
}

func popNext() (string, bool) {
	if len(queue) == 0 {
		return "", false
	}
	u := queue[0]
	queue = queue[1:]
	delete(qSeen, u)
	return u, true
}

func pathSegments(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func slugReserved(seg string) bool {
	switch strings.ToLower(strings.TrimSpace(seg)) {
	case "admin", "api", "static", "health", "posts", "rss-feeds", "login", "rss",
		"halaman", "tag", "author", "favicon.ico", "robots.txt", "ads.txt":
		return true
	default:
		return false
	}
}

// PostSlugFromPublicPathOrURL returns the article slug segment from a full URL or a root-relative path.
func PostSlugFromPublicPathOrURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		segs := pathSegments(raw)
		if len(segs) == 0 {
			return ""
		}
		slug := segs[len(segs)-1]
		if slugReserved(slug) {
			return ""
		}
		return slug
	}
	u, err := url.Parse(raw)
	if err != nil || u.Path == "" {
		return ""
	}
	segs := pathSegments(u.Path)
	if len(segs) == 0 {
		return ""
	}
	slug := segs[len(segs)-1]
	if slugReserved(slug) {
		return ""
	}
	return slug
}

// RunOneURL updates published post updated_at when slug matches, then optionally GETs absolute URLs.
func RunOneURL(ctx context.Context, db *gorm.DB, client *http.Client, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	now := time.Now().UTC()
	repo := repositories.NewPostRepository(db)
	slug := PostSlugFromPublicPathOrURL(raw)
	if slug != "" {
		ok, err := repo.TouchPublishedUpdatedAtBySlug(slug, now)
		if err != nil {
			log.Printf("post date url recycle: touch slug=%q: %v", slug, err)
		} else if ok {
			log.Printf("post date url recycle: updated post slug=%q", slug)
		}
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("post date url recycle: GET %q: %v", raw, err)
		return
	}
	_ = resp.Body.Close()
}

// TickScheduled drains at most one URL per call from the in-memory queue when enabled,
// spacing pops by interval minutes. Disabling the feature resets the queue.
func TickScheduled(db *gorm.DB) {
	if db == nil {
		return
	}
	ss := repositories.SiteSettingOrDefault(db)
	if !ss.PostDateUrlRecycleEnabled {
		ResetQueue()
		return
	}
	intervalMins := ss.PostDateUrlRecycleIntervalMins
	if intervalMins < 0 {
		intervalMins = 0
	}
	if intervalMins > 1440 {
		intervalMins = 1440
	}
	gap := time.Duration(intervalMins) * time.Minute

	qMu.Lock()
	now := time.Now()
	if !qLastAt.IsZero() && gap > 0 && now.Sub(qLastAt) < gap {
		qMu.Unlock()
		return
	}
	u, ok := popNext()
	if !ok {
		qMu.Unlock()
		return
	}
	qLastAt = now
	qMu.Unlock()

	RunOneURL(context.Background(), db, nil, u)
}
