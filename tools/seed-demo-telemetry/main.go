// Demo seed for the telemetry panels: ~30 days of logins, heartbeats, page
// views, note opens and flashcard review events for a handful of demo
// students. Written straight to the SQLite database because API writes are
// always stamped "now" — history can only be backfilled at the DB level.
//
//   go run ./tools/seed-demo-telemetry
//
// Deterministic (seeded RNG per student+day) and idempotent at day
// granularity: a student/day that already has a heartbeat is skipped, so
// re-running extends the demo window instead of duplicating rows.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"

	"ap-psych-final/backend/backend"
)

const (
	windowDays = 30
	demoDomain = "tsinglan.org"
	password   = "demo-activity-2026"
)

var demoStudents = []struct{ Name, Email string }{
	{"Demo Student", "demo-student@" + demoDomain},
	{"Alice Wu", "demo-alice@" + demoDomain},
	{"Brian Chen", "demo-brian@" + demoDomain},
	{"Cindy Liu", "demo-cindy@" + demoDomain},
	{"David Zhao", "demo-david@" + demoDomain},
	{"Emma Sun", "demo-emma@" + demoDomain},
}

// diligence shapes how often each student shows up: Alice every day, David
// barely at all. Index-aligned with demoStudents.
var diligence = []float64{0.85, 0.95, 0.6, 0.75, 0.25, 0.5}

var pagePool = []string{
	"page.dashboard", "page.flashcards", "page.notes", "page.practice",
	"page.terms", "page.wrongbook", "page.writing", "page.practice-history",
}

var userAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/128.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/17.5",
	"Mozilla/5.0 (iPad; CPU OS 17_5) Mobile/15E148 Safari/604.1",
}

func localLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*3600)
}

func id(prefix string, r *rand.Rand) string {
	const hex = "0123456789abcdef"
	var b [12]byte
	for i := range b {
		b[i] = hex[r.Intn(16)]
	}
	return prefix + "_" + string(b[:])
}

func seedFor(email string, day int) int64 {
	var h int64 = 2166136261
	for _, c := range email + fmt.Sprintf("#%d", day) {
		h = (h ^ int64(c)) * 16777619
	}
	if h < 0 {
		h = -h
	}
	return h
}

func metaJSON(fields map[string]any) datatypes.JSON {
	raw, _ := json.Marshal(fields)
	return datatypes.JSON(raw)
}

func main() {
	loc := localLocation()
	app, err := backend.NewApp(filepath.Join("data", "app.db"), filepath.Join("data", "sources"))
	if err != nil {
		log.Fatal(err)
	}
	db := app.DB

	// Concepts and note resources give review events and note.open meta real
	// foreign keys so the aggregates join cleanly.
	var conceptIDs []string
	db.Raw("select id from concepts order by id limit 80").Scan(&conceptIDs)
	var noteIDs []struct {
		ID    string
		Title string
	}
	db.Raw("select id, title from note_resources order by position limit 5").Scan(&noteIDs)

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	today := time.Now().In(loc)
	inserted, skipped := 0, 0
	for si, demo := range demoStudents {
		email := strings.ToLower(demo.Email)
		var user backend.User
		if err := db.First(&user, "email = ?", email).Error; err != nil {
			user = backend.User{
				ID: id("usr", rand.New(rand.NewSource(seedFor(email, 0)))),
				TenantID: "school", Name: demo.Name, Email: email, Role: "student",
				Provider: "local", PasswordHash: string(hash),
			}
			if err := db.Create(&user).Error; err != nil {
				log.Fatalf("create %s: %v", email, err)
			}
			fmt.Printf("created student %s (password %s)\n", email, password)
		}

		var lastLogin, lastSeen *time.Time
		for day := windowDays - 1; day >= 0; day-- {
			date := today.AddDate(0, 0, -day)
			dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)

			var existing int64
			db.Raw("select count(*) from activity_events where user_id = ? and created_at >= ? and created_at < ?",
				user.ID, dayStart, dayStart.AddDate(0, 0, 1)).Scan(&existing)
			if existing > 0 {
				skipped++
				continue
			}

			r := rand.New(rand.NewSource(seedFor(email, day)))
			weekday := dayStart.Weekday()
			chance := diligence[si]
			if weekday == time.Saturday || weekday == time.Sunday {
				chance *= 0.5
			}
			if r.Float64() > chance {
				continue
			}

			// One session, evening-weighted, 25-85 minutes long.
			startHour := 8 + r.Intn(13)
			if r.Float64() < 0.65 {
				startHour = 18 + r.Intn(4)
			}
			start := dayStart.Add(time.Duration(startHour)*time.Hour + time.Duration(r.Intn(50))*time.Minute)
			length := time.Duration(25+r.Intn(60)) * time.Minute
			ua := userAgentPool[r.Intn(len(userAgentPool))]
			ip := fmt.Sprintf("192.168.10.%d", 20+r.Intn(200))

			loginAt := start
			db.Create(&backend.ActivityEvent{
				ID: id("act", r), TenantID: "school", UserID: user.ID, Type: "login", Name: "login.success",
				Path: "/api/auth/login",
				Meta:  metaJSON(map[string]any{"provider": "local", "email": email, "ip": ip, "ua": ua}),
				CreatedAt: loginAt,
			})
			inserted++

			for beat := start; beat.Before(start.Add(length)); beat = beat.Add(5 * time.Minute) {
				db.Create(&backend.ActivityEvent{
					ID: id("act", r), TenantID: "school", UserID: user.ID, Type: "heartbeat", Name: "heartbeat",
					Path: "/api/dashboard", CreatedAt: beat,
				})
				inserted++
			}

			// Page views across the session.
			views := 2 + r.Intn(5)
			for i := 0; i < views; i++ {
				at := start.Add(time.Duration(r.Intn(int(length.Minutes()))) * time.Minute)
				page := "page.dashboard"
				if i > 0 {
					page = pagePool[r.Intn(len(pagePool))]
				}
				db.Create(&backend.ActivityEvent{
					ID: id("act", r), TenantID: "school", UserID: user.ID, Type: "page_view", Name: page,
					CreatedAt: at,
				})
				inserted++
				if page == "page.notes" && len(noteIDs) > 0 && r.Float64() < 0.7 {
					note := noteIDs[r.Intn(len(noteIDs))]
					db.Create(&backend.ActivityEvent{
						ID: id("act", r), TenantID: "school", UserID: user.ID, Type: "feature", Name: "note.open",
						Meta:      metaJSON(map[string]any{"resourceId": note.ID, "title": note.Title}),
						CreatedAt: at.Add(time.Duration(r.Intn(10)) * time.Minute),
					})
					inserted++
				}
			}

			// Flashcard reviews with a realistic self-assessment mix.
			reviewCount := 4 + r.Intn(38)
			for i := 0; i < reviewCount && len(conceptIDs) > 0; i++ {
				at := start.Add(time.Duration(r.Intn(int(length.Minutes()))) * time.Minute)
				roll := r.Float64()
				response := "proficient"
				if roll > 0.85 {
					response = "unknown"
				} else if roll > 0.55 {
					response = "fuzzy"
				}
				db.Create(&backend.ReviewEvent{
					ID: id("rev", r), UserID: user.ID, ConceptID: conceptIDs[r.Intn(len(conceptIDs))],
					Response: response, DurationMS: 1500 + r.Intn(12500), CreatedAt: at,
				})
				inserted++
			}

			seen := start.Add(length)
			if lastLogin == nil || loginAt.After(*lastLogin) {
				moment := loginAt
				lastLogin = &moment
			}
			if lastSeen == nil || seen.After(*lastSeen) {
				moment := seen
				lastSeen = &moment
			}
		}
		if lastLogin != nil {
			db.Exec("update users set last_login_at = ?, last_seen_at = ? where id = ? and (last_login_at is null or last_login_at < ?)", lastLogin, lastSeen, user.ID, lastLogin)
		}
	}
	fmt.Printf("done: %d events inserted, %d student/days skipped (already seeded)\n", inserted, skipped)
}
