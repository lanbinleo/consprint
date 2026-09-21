package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Telemetry event types recorded in activity_events.
const (
	EventLogin     = "login"
	EventHeartbeat = "heartbeat"
	EventPageView  = "page_view"
	EventFeature   = "feature"
)

// heartbeatWindow is the dedupe window for session heartbeats: one row per
// user per window, no matter how many API calls they make. A var (not const)
// so tests can shrink it.
var heartbeatWindow = 5 * time.Minute

// recordActivity appends one telemetry row. Telemetry must never fail a
// user-facing request, so errors are logged and skipped.
func (a *App) recordActivity(userID, eventType, name, path string, meta map[string]any) {
	var metaJSON datatypes.JSON
	if len(meta) > 0 {
		if raw, err := json.Marshal(meta); err == nil {
			metaJSON = datatypes.JSON(raw)
		}
	}
	tenant := schoolTenantID
	if userID != "" {
		var user User
		if err := a.DB.Select("tenant_id").First(&user, "id = ?", userID).Error; err == nil && user.TenantID != "" {
			tenant = user.TenantID
		}
	}
	event := ActivityEvent{
		ID: NewID("act"), TenantID: tenant, UserID: userID,
		Type: eventType, Name: name, Path: path, Meta: metaJSON, CreatedAt: appNow(),
	}
	if err := a.DB.Create(&event).Error; err != nil {
		log.Printf("telemetry write failed: %v", err)
	}
}

// recordLogin appends the outcome of a sign-in attempt with just enough
// context (provider, attempted email, client IP, trimmed user-agent) for the
// admin-only security view.
func (a *App) recordLogin(c *gin.Context, userID, provider, email string, success bool) {
	name := "login.failed"
	if success {
		name = "login.success"
	}
	a.recordActivity(userID, EventLogin, name, c.Request.URL.Path, map[string]any{
		"provider": provider,
		"email":    email,
		"ip":       c.ClientIP(),
		"ua":       trimUserAgent(c.GetHeader("User-Agent")),
	})
}

// touchLastLogin records the successful sign-in timestamp on the user row.
func (a *App) touchLastLogin(userID string) {
	now := appNow()
	a.DB.Model(&User{}).Where("id = ?", userID).Update("last_login_at", now)
}

func trimUserAgent(ua string) string {
	if len(ua) > 160 {
		ua = ua[:160]
	}
	return ua
}

type heartbeatTracker struct {
	mu     sync.Mutex
	seenAt map[string]time.Time
}

// heartbeat records at most one activity event per user per window (after the
// request succeeded) so the admin panels can answer "who was online when"
// without logging every API call. The map is process-local and school-scale;
// a restart at worst records one extra row per user.
func (a *App) heartbeat() gin.HandlerFunc {
	tracker := &heartbeatTracker{seenAt: map[string]time.Time{}}
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() >= 400 {
			return
		}
		userID := c.GetString("userID")
		if userID == "" {
			return
		}
		now := time.Now()
		tracker.mu.Lock()
		if last, ok := tracker.seenAt[userID]; ok && now.Sub(last) < heartbeatWindow {
			tracker.mu.Unlock()
			return
		}
		tracker.seenAt[userID] = now
		tracker.mu.Unlock()
		// FullPath is the matched route pattern ("/api/practice/attempts/:id"),
		// i.e. path segments are already normalized for aggregation.
		a.recordActivity(userID, EventHeartbeat, "heartbeat", c.FullPath(), nil)
		a.DB.Model(&User{}).Where("id = ?", userID).Update("last_seen_at", appNow())
	}
}

// telemetryNameRE bounds client-supplied event names to a safe, sortable set.
var telemetryNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type telemetryEventIn struct {
	Type string         `json:"type"`
	Name string         `json:"name"`
	Path string         `json:"path"`
	Meta map[string]any `json:"meta"`
}

// reportTelemetry accepts small batches of page_view / feature events from
// the frontend. Unknown types or malformed names are skipped silently — a
// chatty or stale client must not error out.
func (a *App) reportTelemetry(c *gin.Context) {
	var req struct {
		Events []telemetryEventIn `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Events) == 0 {
		c.JSON(400, gin.H{"error": "events required"})
		return
	}
	if len(req.Events) > 50 {
		c.JSON(400, gin.H{"error": "at most 50 events per batch"})
		return
	}
	userID := c.GetString("userID")
	tenant := c.GetString("tenantID")
	if tenant == "" {
		tenant = schoolTenantID
	}
	now := appNow()
	rows := make([]ActivityEvent, 0, len(req.Events))
	for _, in := range req.Events {
		if in.Type != EventPageView && in.Type != EventFeature {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(in.Name))
		if !telemetryNameRE.MatchString(name) {
			continue
		}
		var metaJSON datatypes.JSON
		if len(in.Meta) > 0 {
			if raw, err := json.Marshal(in.Meta); err == nil {
				metaJSON = datatypes.JSON(raw)
			}
		}
		if len(in.Path) > 160 {
			in.Path = in.Path[:160]
		}
		rows = append(rows, ActivityEvent{
			ID: NewID("act"), TenantID: tenant, UserID: userID,
			Type: in.Type, Name: name, Path: strings.TrimSpace(in.Path), Meta: metaJSON, CreatedAt: now,
		})
	}
	if len(rows) == 0 {
		c.JSON(200, gin.H{"ok": true, "stored": 0})
		return
	}
	if err := a.DB.Create(&rows).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not store events"})
		return
	}
	c.JSON(200, gin.H{"ok": true, "stored": len(rows)})
}

// pruneTelemetry drops deduped heartbeat rows older than the retention window
// (default 180 days; TELEMETRY_RETENTION_DAYS=0 keeps everything). Heartbeats
// are the only high-volume event type — login/page/feature rows stay forever.
func pruneTelemetry(db *gorm.DB) {
	days := envInt("TELEMETRY_RETENTION_DAYS", 180)
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	db.Where("type = ? and created_at < ?", EventHeartbeat, cutoff).Delete(&ActivityEvent{})
}

// tzModifier returns the SQLite time modifier that shifts a stored UTC
// timestamp into APP_TIMEZONE, e.g. "+28800 seconds" for Asia/Shanghai. Used
// with strftime so day/hour buckets match the school's local calendar.
func tzModifier() string {
	_, offset := appNow().Zone()
	if offset < 0 {
		return fmt.Sprintf("-%d seconds", -offset)
	}
	return fmt.Sprintf("+%d seconds", offset)
}
