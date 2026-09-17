package backend

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Tenant struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"not null" json:"name"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type User struct {
	ID            string         `gorm:"primaryKey" json:"id"`
	TenantID      string         `gorm:"index;not null" json:"tenantId"`
	Name          string         `gorm:"not null" json:"name"`
	Email         string         `gorm:"uniqueIndex;not null" json:"email"`
	Role          string         `gorm:"not null;default:student" json:"role"` // student | teacher | admin
	Provider      string         `gorm:"not null;default:local" json:"provider"`
	EntraOID      *string        `gorm:"uniqueIndex" json:"-"`
	AvatarDataURL string         `json:"avatarDataUrl"`
	PasswordHash  string         `gorm:"not null" json:"-"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type Course struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"not null" json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Unit struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	CourseID  string    `gorm:"index;not null" json:"courseId"`
	Title     string    `gorm:"not null" json:"title"`
	Position  int       `json:"position"`
	Topics    []Topic   `json:"topics,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Topic struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UnitID    string    `gorm:"index;not null" json:"unitId"`
	Title     string    `gorm:"not null" json:"title"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Concept struct {
	ID             string          `gorm:"primaryKey" json:"id"`
	CourseID       string          `gorm:"index;not null" json:"courseId"`
	UnitID         string          `gorm:"index;not null" json:"unitId"`
	TopicID        string          `gorm:"index;not null" json:"topicId"`
	Term           string          `gorm:"not null" json:"term"`
	NormalizedTerm string          `gorm:"index;not null" json:"normalizedTerm"`
	Position       int             `json:"position"`
	ContentStatus  string          `gorm:"not null;default:pending" json:"contentStatus"`
	Content        *ConceptContent `json:"content,omitempty"`
	Unit           Unit            `json:"unit,omitempty"`
	Topic          Topic           `json:"topic,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type ConceptContent struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	ConceptID   string         `gorm:"uniqueIndex;not null" json:"conceptId"`
	Definition  datatypes.JSON `json:"definition"`
	Examples    datatypes.JSON `json:"examples"`
	Pitfalls    datatypes.JSON `json:"pitfalls"`
	Notes       datatypes.JSON `json:"notes"`
	Source      string         `json:"source"`
	Confidence  float64        `json:"confidence"`
	NeedsReview bool           `json:"needsReview"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// UserConceptState tracks each learner's three-tier self-assessment per concept.
// Status is "" (not yet marked), "proficient", "fuzzy", or "unknown".
type UserConceptState struct {
	ID              string     `gorm:"primaryKey" json:"id"`
	UserID          string     `gorm:"uniqueIndex:idx_user_concept;not null" json:"userId"`
	ConceptID       string     `gorm:"uniqueIndex:idx_user_concept;not null" json:"conceptId"`
	Status          string     `gorm:"index;not null;default:''" json:"status"`
	ReviewCount     int        `json:"reviewCount"`
	ShortTermReview bool       `json:"shortTermReview"`
	LastReviewedAt  *time.Time `json:"lastReviewedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// ReviewEvent is the append-only log of flashcard self-assessments.
type ReviewEvent struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"index;not null" json:"userId"`
	ConceptID  string    `gorm:"index;not null" json:"conceptId"`
	Response   string    `gorm:"index;not null" json:"response"` // proficient | fuzzy | unknown
	DurationMS int       `json:"durationMs"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ImportRun struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Source    string    `gorm:"index;not null" json:"source"`
	Status    string    `gorm:"not null" json:"status"`
	Message   string    `json:"message"`
	Counts    string    `json:"counts"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
