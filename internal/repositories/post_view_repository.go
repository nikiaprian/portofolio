package repositories

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nikiaprian/tirtayasa/internal/cache"
	"github.com/nikiaprian/tirtayasa/internal/entity"
	"github.com/nikiaprian/tirtayasa/internal/helper/publishedpost"
)

// PostViewRank is a post_id with its view count from aggregated post_views rows.
type PostViewRank struct {
	PostID uint
	Views  int64
}

// PostViewRepository handles post_views persistence and aggregated ranking queries.
type PostViewRepository interface {
	RecordUniqueView(postID uint, viewerIP string, viewDateUTC string) (bool, error)
	CountForPost(postID uint) (int64, error)
	CountForPosts(postIDs []uint) (map[uint]int64, error)
	RankSince(cutoff time.Time, limit int) ([]PostViewRank, error)
	RankInCategory(categoryID, excludePostID uint, limit int) ([]PostViewRank, error)
	RankPublishedAllTime(excludePostIDs []uint, limit int) ([]PostViewRank, error)
}

type postViewRepository struct {
	db *gorm.DB
}

func NewPostViewRepository(db *gorm.DB) PostViewRepository {
	return &postViewRepository{db: db}
}

func viewDateOnOrAfter(cutoff time.Time) string {
	return cutoff.UTC().Format("2006-01-02")
}

func (r *postViewRepository) RecordUniqueView(postID uint, viewerIP string, viewDateUTC string) (bool, error) {
	if postID == 0 || strings.TrimSpace(viewDateUTC) == "" {
		return false, nil
	}
	ip := strings.TrimSpace(viewerIP)
	if ip == "" {
		return false, nil
	}
	if len(ip) > 45 {
		ip = ip[:45]
	}

	pv := entity.PostView{PostID: postID, ViewerIP: ip, ViewDate: viewDateUTC}
	res := r.db.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}, {Name: "viewer_ip"}, {Name: "view_date"}},
			DoNothing: true,
		}).
		Create(&pv)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *postViewRepository) CountForPost(postID uint) (int64, error) {
	if postID == 0 {
		return 0, nil
	}
	var n int64
	err := r.db.Model(&entity.PostView{}).Where(&entity.PostView{PostID: postID}).Count(&n).Error
	return n, err
}

func (r *postViewRepository) CountForPosts(postIDs []uint) (map[uint]int64, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	var rows []postViewRankRow
	err := postViewRankQuery(r.db).
		Where(clause.IN{Column: "post_id", Values: uintsToInterfaces(postIDs)}).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]int64, len(rows))
	for _, row := range rows {
		out[row.PostID] = row.Views
	}
	return out, nil
}

type postViewRankRow struct {
	PostID uint  `gorm:"column:post_id"`
	Views  int64 `gorm:"column:views"`
}

func postViewRankQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&entity.PostView{}).
		Select("post_id, COUNT(*) AS views").
		Group("post_id").
		Order("views DESC")
}

func (r *postViewRepository) listPublishedPostIDs(categoryID, excludePostID uint, excludePostIDs []uint) ([]uint, error) {
	var ids []uint
	q := r.db.Model(&entity.Post{}).
		Select("id").
		Scopes(publishedpost.ScopeVisiblePublished("", time.Now()))
	if categoryID > 0 {
		q = q.Where(&entity.Post{CategoryID: categoryID})
	}
	if excludePostID > 0 {
		q = q.Not(&entity.Post{ID: excludePostID})
	}
	if len(excludePostIDs) > 0 {
		q = q.Not(clause.IN{Column: "id", Values: uintsToInterfaces(excludePostIDs)})
	}
	err := q.Pluck("id", &ids).Error
	return ids, err
}

func rankPostViewsByPostIDs(db *gorm.DB, postIDs []uint, limit int) ([]PostViewRank, error) {
	if len(postIDs) == 0 || limit <= 0 {
		return nil, nil
	}
	q := postViewRankQuery(db).
		Where(clause.IN{Column: "post_id", Values: uintsToInterfaces(postIDs)}).
		Limit(limit)
	return scanPostViewRanks(q, limit)
}

func uintsToInterfaces(ids []uint) []interface{} {
	out := make([]interface{}, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func scanPostViewRanks(db *gorm.DB, limit int) ([]PostViewRank, error) {
	if limit <= 0 {
		return nil, nil
	}
	var rows []postViewRankRow
	if err := db.Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]PostViewRank, len(rows))
	for i, row := range rows {
		out[i] = PostViewRank{PostID: row.PostID, Views: row.Views}
	}
	return out, nil
}

const postViewRankCacheTTL = 60 * time.Second

const postViewRankRedisPrefix = "data:post_view_rank:"

var (
	postViewRankCacheMu sync.Mutex
	postViewRankCacheL1 = map[string]cachedPostViewRanks{}
	postViewRankFlight  singleflight.Group
)

type cachedPostViewRanks struct {
	rows      []PostViewRank
	expiresAt time.Time
}

func getCachedPostViewRanksL1(key string) ([]PostViewRank, bool) {
	postViewRankCacheMu.Lock()
	defer postViewRankCacheMu.Unlock()
	e, ok := postViewRankCacheL1[key]
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			delete(postViewRankCacheL1, key)
		}
		return nil, false
	}
	return e.rows, true
}

func setCachedPostViewRanksL1(key string, rows []PostViewRank) {
	postViewRankCacheMu.Lock()
	defer postViewRankCacheMu.Unlock()
	postViewRankCacheL1[key] = cachedPostViewRanks{
		rows:      rows,
		expiresAt: time.Now().Add(postViewRankCacheTTL),
	}
}

func getCachedPostViewRanksFromRedis(key string) ([]PostViewRank, bool) {
	store := cache.GlobalStore()
	if store == nil {
		return nil, false
	}
	var rows []PostViewRank
	ok, err := store.GetJSON(context.Background(), postViewRankRedisPrefix+key, &rows)
	if err != nil || !ok {
		return nil, false
	}
	return rows, true
}

func setCachedPostViewRanks(key string, rows []PostViewRank) {
	setCachedPostViewRanksL1(key, rows)
	if store := cache.GlobalStore(); store != nil {
		_ = store.SetJSON(context.Background(), postViewRankRedisPrefix+key, rows, postViewRankCacheTTL)
	}
}

func (r *postViewRepository) cachedRank(key string, query func() ([]PostViewRank, error)) ([]PostViewRank, error) {
	if rows, ok := getCachedPostViewRanksL1(key); ok {
		return rows, nil
	}
	if rows, ok := getCachedPostViewRanksFromRedis(key); ok {
		setCachedPostViewRanksL1(key, rows)
		return rows, nil
	}
	v, err, _ := postViewRankFlight.Do(key, func() (any, error) {
		if rows, ok := getCachedPostViewRanksL1(key); ok {
			return rows, nil
		}
		if rows, ok := getCachedPostViewRanksFromRedis(key); ok {
			setCachedPostViewRanksL1(key, rows)
			return rows, nil
		}
		rows, err := query()
		if err != nil {
			return nil, err
		}
		setCachedPostViewRanks(key, rows)
		return rows, nil
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.([]PostViewRank), nil
}

func postViewRankAllTimeKey(excludePostIDs []uint, limit int) string {
	if len(excludePostIDs) == 0 {
		return fmt.Sprintf("all::%d", limit)
	}
	sorted := append([]uint(nil), excludePostIDs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	parts := make([]string, len(sorted))
	for i, id := range sorted {
		parts[i] = strconv.FormatUint(uint64(id), 10)
	}
	return fmt.Sprintf("all:%s:%d", strings.Join(parts, ","), limit)
}

func (r *postViewRepository) RankSince(cutoff time.Time, limit int) ([]PostViewRank, error) {
	if limit <= 0 {
		return nil, nil
	}
	minDate := viewDateOnOrAfter(cutoff)
	key := fmt.Sprintf("since:%s:%d", minDate, limit)
	return r.cachedRank(key, func() ([]PostViewRank, error) {
		q := postViewRankQuery(r.db).
			Where(clause.Gte{Column: "view_date", Value: minDate})
		return scanPostViewRanks(q, limit)
	})
}

func (r *postViewRepository) RankInCategory(categoryID, excludePostID uint, limit int) ([]PostViewRank, error) {
	if categoryID == 0 || limit <= 0 {
		return nil, nil
	}
	key := fmt.Sprintf("cat:%d:%d:%d", categoryID, excludePostID, limit)
	return r.cachedRank(key, func() ([]PostViewRank, error) {
		ids, err := r.listPublishedPostIDs(categoryID, excludePostID, nil)
		if err != nil {
			return nil, err
		}
		return rankPostViewsByPostIDs(r.db, ids, limit)
	})
}

func (r *postViewRepository) RankPublishedAllTime(excludePostIDs []uint, limit int) ([]PostViewRank, error) {
	if limit <= 0 {
		return nil, nil
	}
	key := postViewRankAllTimeKey(excludePostIDs, limit)
	return r.cachedRank(key, func() ([]PostViewRank, error) {
		ids, err := r.listPublishedPostIDs(0, 0, excludePostIDs)
		if err != nil {
			return nil, err
		}
		return rankPostViewsByPostIDs(r.db, ids, limit)
	})
}
