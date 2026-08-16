package worker

import (
	"context"
	"log"
	"time"

	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavSyncWorker struct {
	db          *gorm.DB
	amfiService *services.AmfiService
	interval    time.Duration
	stopCh      chan struct{}
}

func NewNavSyncWorker(db *gorm.DB, amfi *services.AmfiService, interval time.Duration) *NavSyncWorker {
	if interval <= 0 {
		interval = 4 * time.Hour
	}
	return &NavSyncWorker{
		db:          db,
		amfiService: amfi,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

func (w *NavSyncWorker) Start() {
	log.Println("NAV Sync Background Worker started.")
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		// Run one sync on startup after 10 seconds
		time.Sleep(10 * time.Second)
		w.runSync()

		for {
			select {
			case <-ticker.C:
				w.runSync()
			case <-w.stopCh:
				log.Println("NAV Sync Background Worker stopped.")
				return
			}
		}
	}()
}

func (w *NavSyncWorker) Stop() {
	close(w.stopCh)
}

func (w *NavSyncWorker) runSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	log.Println("NavSyncWorker: Fetching daily NAV snapshot from AMFI...")
	snapshots, err := w.amfiService.FetchLatestNavSnapshots(ctx)
	if err != nil {
		log.Printf("NavSyncWorker error: %v", err)
		return
	}

	log.Printf("NavSyncWorker: Downloaded %d latest NAV snapshots. Updating database...", len(snapshots))

	nowIst := services.IndiaNow()
	nowUtc := time.Now().UTC()

	var navHistoryBatch []models.NavHistory
	var trackerBatch []models.NavSyncTracker

	for _, snap := range snapshots {
		navHistoryBatch = append(navHistoryBatch, models.NavHistory{
			SchemeCode: snap.SchemeCode,
			Date:       snap.Date,
			Nav:        snap.Nav,
		})

		navDate := snap.Date
		navVal := snap.Nav
		trackerBatch = append(trackerBatch, models.NavSyncTracker{
			SchemeCode:              snap.SchemeCode,
			LatestNav:               &navVal,
			LatestNavDate:           &navDate,
			LastLatestNavCheckAtIst: &nowIst,
			UpdatedAtUtc:            nowUtc,
		})

		if len(navHistoryBatch) >= 1000 {
			w.flushBatch(ctx, navHistoryBatch, trackerBatch)
			navHistoryBatch = nil
			trackerBatch = nil
		}
	}

	if len(navHistoryBatch) > 0 {
		w.flushBatch(ctx, navHistoryBatch, trackerBatch)
	}

	log.Println("NavSyncWorker: Daily NAV sync completed successfully.")
}

func (w *NavSyncWorker) flushBatch(ctx context.Context, navs []models.NavHistory, trackers []models.NavSyncTracker) {
	_ = w.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "scheme_code"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"nav"}),
	}).Create(&navs).Error

	_ = w.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "scheme_code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"latest_nav",
			"latest_nav_date",
			"last_latest_nav_check_at_ist",
			"updated_at",
		}),
	}).Create(&trackers).Error
}
