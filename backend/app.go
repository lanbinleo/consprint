package backend

import (
	"os"
	"path/filepath"

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

func NewApp(dbPath, sources string) (*App, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Tenant{}, &User{}, &Course{}, &Unit{}, &Topic{}, &Concept{}, &ConceptContent{}, &UserConceptState{}, &ReviewEvent{}, &ImportRun{}); err != nil {
		return nil, err
	}
	db.Model(&User{}).Where("role = '' OR role IS NULL").Update("role", "student")
	var admins int64
	db.Model(&User{}).Where("role = ?", "admin").Count(&admins)
	if admins == 0 {
		var first User
		if err := db.Order("created_at asc").First(&first).Error; err == nil {
			db.Model(&first).Update("role", "admin")
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
