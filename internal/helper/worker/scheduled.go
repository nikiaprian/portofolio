package worker

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/alsoread"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post/daterecycle"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/social"
)

// StartPublishDueWorker runs a background ticker that promotes scheduled posts whose
// publish_at is in the past to status published.
func StartPublishDueWorker(db *gorm.DB, autoPoster *social.AutoPoster) {
	if db == nil {
		return
	}
	go func() {
		var lastPostDateRefreshAt time.Time
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		repo := repositories.NewPostRepository(db)
		siteSettingRepo := repositories.NewSiteSettingRepository(db)
		for range ticker.C {
			cutoff := time.Now().UTC()
			due, _ := repo.FindScheduledDue(cutoff, 300)
			n, err := repo.PublishDueScheduledPosts(cutoff)
			if err != nil {
				log.Printf("scheduled posts: publish due: %v", err)
			} else if n > 0 {
				log.Printf("scheduled posts: published %d post(s)", n)
				for _, row := range due {
					full, getErr := repo.FindByID(row.ID)
					if getErr != nil || full == nil {
						continue
					}
					if err := alsoread.ApplyForPost(db, full); err != nil {
						log.Printf("scheduled posts: baca-juga reconcile post %d: %v", row.ID, err)
						continue
					}
					if err := repo.Update(full); err != nil {
						log.Printf("scheduled posts: baca-juga save post %d: %v", row.ID, err)
					}
				}
				if autoPoster != nil {
					for _, p := range due {
						p.Status = "published"
						_ = autoPoster.OnPublishedPost(context.Background(), &p)
					}
				}
			}

			ss, ssErr := siteSettingRepo.Get()
			if ssErr == nil && ss != nil && ss.PostDateRefreshEnabled && ss.PostDateRefreshDays > 0 {
				oldCutoff := cutoff.AddDate(0, 0, -ss.PostDateRefreshDays)
				intervalMins := ss.PostDateIntervalMins
				if intervalMins < 0 {
					intervalMins = 0
				}

				if intervalMins > 0 {
					minGap := time.Duration(intervalMins) * time.Minute
					if lastPostDateRefreshAt.IsZero() || cutoff.Sub(lastPostDateRefreshAt) >= minGap {
						ok, refreshErr := repo.RefreshOneOldPublishedDate(oldCutoff, cutoff)
						if refreshErr != nil {
							log.Printf("scheduled posts: refresh old post dates: %v", refreshErr)
						} else if ok {
							lastPostDateRefreshAt = cutoff
							log.Printf("scheduled posts: refreshed date for 1 old published post (interval %d min until next)", intervalMins)
						}
					}
				} else {
					refreshed, refreshErr := repo.RefreshOldPublishedDatesBatch(oldCutoff, cutoff, 200)
					if refreshErr != nil {
						log.Printf("scheduled posts: refresh old post dates: %v", refreshErr)
					} else if refreshed > 0 {
						log.Printf("scheduled posts: refreshed date for %d old published post(s) (batch, no interval)", refreshed)
					}
				}
			}

			daterecycle.TickScheduled(db)
		}
	}()
}
