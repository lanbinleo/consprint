package backend

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type App struct {
	DB        *gorm.DB
	JWTSecret []byte
	Sources   string
	// PublicDir holds shareable note files (PDFs, images) served at /files.
	// It lives next to the database so test apps stay inside t.TempDir().
	PublicDir string
}

type Claims struct {
	UserID   string `json:"userId"`
	TenantID string `json:"tenantId"`
	jwt.RegisteredClaims
}

// schoolTenantID is the single school tenant every account belongs to.
const schoolTenantID = "school"

func NewApp(dbPath, sources string) (*App, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	publicDir := filepath.Join(filepath.Dir(dbPath), "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		return nil, err
	}
	// WAL + busy_timeout so concurrent requests (reviews, practice submits,
	// analytics) don't fail with SQLITE_BUSY under load.
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Tenant{}, &User{}, &Course{}, &Unit{}, &Topic{}, &Concept{}, &ConceptContent{}, &UserConceptState{}, &ReviewEvent{}, &ImportRun{}, &Tag{}, &Question{}, &PracticeSet{}, &PracticeSetItem{}, &PracticeAttempt{}, &PracticeAnswer{}, &NoteResource{}, &NoteResourceItem{}, &Announcement{}, &CalendarEvent{}, &Quote{}); err != nil {
		return nil, err
	}
	migrateLegacyEntraOID(db)
	migrateCalendarKinds(db)
	if err := ensureSchoolTenant(db); err != nil {
		return nil, err
	}
	db.Model(&User{}).Where("role = '' OR role IS NULL").Update("role", "student")
	emails := adminEmails()
	if len(emails) > 0 {
		db.Model(&User{}).Where("email in ?", emails).Update("role", "admin")
	} else if !productionMode() {
		// Fallback for local development: keep the app usable with an admin
		// account even when ADMIN_EMAILS is not configured. Never in
		// production, where the first arbitrary registrant must not become
		// admin — configure ADMIN_EMAILS or bootstrap via Entra instead.
		var admins int64
		db.Model(&User{}).Where("role = ?", "admin").Count(&admins)
		if admins == 0 {
			var first User
			if err := db.Order("created_at asc").First(&first).Error; err == nil {
				db.Model(&first).Update("role", "admin")
			}
		}
	}
	app := &App{DB: db, JWTSecret: []byte(env("JWT_SECRET", "local-dev-secret-change-me")), Sources: sources, PublicDir: publicDir}
	var count int64
	db.Model(&Concept{}).Count(&count)
	if count == 0 {
		if err := (Importer{DB: db, Sources: sources}).RunAll(); err != nil {
			log.Printf("initial source import failed: %v", err)
		}
	}
	if err := seedNoteResources(db); err != nil {
		return nil, err
	}
	if err := seedBoards(db); err != nil {
		return nil, err
	}
	return app, nil
}

// migrateCalendarKinds folds the retired generic "assessment" kind into the
// more specific "exam" (midterm/final). Idempotent.
func migrateCalendarKinds(db *gorm.DB) {
	db.Exec("update calendar_events set kind = 'exam' where kind = 'assessment'")
}

func ensureSchoolTenant(db *gorm.DB) error {
	var existing int64
	db.Model(&Tenant{}).Where("id = ?", schoolTenantID).Count(&existing)
	if existing > 0 {
		return nil
	}
	name := env("SCHOOL_NAME", "AP Psychology")
	return db.Create(&Tenant{ID: schoolTenantID, Name: name}).Error
}

// migrateLegacyEntraOID moves data out of the mis-named entra_o_id column
// created by early builds (GORM snake-cased EntraOID as entra_o_id, while
// queries expect entra_oid). Idempotent: runs only while the legacy column
// exists.
func migrateLegacyEntraOID(db *gorm.DB) {
	var legacy int64
	db.Raw("select count(*) from pragma_table_info('users') where name = 'entra_o_id'").Scan(&legacy)
	if legacy == 0 {
		return
	}
	db.Exec("update users set entra_oid = entra_o_id where entra_oid is null and entra_o_id is not null")
	db.Exec("drop index if exists idx_users_entra_o_id")
	db.Exec("alter table users drop column entra_o_id")
}

func adminEmails() []string {
	var out []string
	for _, part := range strings.Split(env("ADMIN_EMAILS", ""), ",") {
		if v := strings.ToLower(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func isAdminEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	for _, candidate := range adminEmails() {
		if candidate == email {
			return true
		}
	}
	return false
}
