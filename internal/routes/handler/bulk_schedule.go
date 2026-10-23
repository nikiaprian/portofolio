package handler

import (
	"strings"
	"time"
)

const bulkScheduleDefaultIntervalMin = 60
const bulkScheduleMinLead = 2 * time.Minute

// bulkScheduledPublishAtUTC returns when a bulk-generated post should go live.
// batchBase is RFC3339 from the client (batch start). titleIndex is the 0-based position in the batch.
// intervalMinutes 0 defaults to bulkScheduleDefaultIntervalMin between posts.
func bulkScheduledPublishAtUTC(batchBase string, titleIndex, intervalMinutes int) time.Time {
	interval := intervalMinutes
	if interval <= 0 {
		interval = bulkScheduleDefaultIntervalMin
	}
	now := time.Now().UTC()
	delta := time.Duration(titleIndex) * time.Duration(interval) * time.Minute

	anchor := now
	s := strings.TrimSpace(batchBase)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		anchor = t.UTC()
	} else if t, err := time.Parse(time.RFC3339, s); err == nil {
		anchor = t.UTC()
	}

	minAnchor := now.Add(bulkScheduleMinLead)
	if anchor.Before(minAnchor) {
		anchor = minAnchor
	}
	at := anchor.Add(delta)
	return at.Truncate(time.Second)
}
