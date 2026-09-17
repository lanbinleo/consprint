package backend

import (
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
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Tenant{}, &User{}, &Course{}, &Unit{}, &Topic{}, &Concept{}, &ConceptContent{}, &UserConceptState{}, &ReviewEvent{}, &ImportRun{}, &Tag{}, &Question{}, &PracticeSet{}, &PracticeSetItem{}, &PracticeAttempt{}, &PracticeAnswer{}); err != nil {
		return nil, err
	}
	if err := ensureSchoolTenant(db); err != nil {
		return nil, err
	}
	db.Model(&User{}).Where("role = '' OR role IS NULL").Update("role", "student")
	emails := adminEmails()
	if len(emails) > 0 {
		db.Model(&User{}).Where("email in ?", emails).Update("role", "admin")
	} else {
		// Fallback for local development: keep the app usable with an admin
		// account even when ADMIN_EMAILS is not configured.
		var admins int64
		db.Model(&User{}).Where("role = ?", "admin").Count(&admins)
		if admins == 0 {
			var first User
			if err := db.Order("created_at asc").First(&first).Error; err == nil {
				db.Model(&first).Update("role", "admin")
			}
		}
	}
	app := &App{DB: db, JWTSecret: []byte(env("JWT_SECRET", "local-dev-secret-change-me")), Sources: sources}
	var count int64
	db.Model(&Concept{}).Count(&count)
	if count == 0 {
		_ = Importer{DB: db, Sources: sources}.RunAll()
	}
	return app, nil
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
