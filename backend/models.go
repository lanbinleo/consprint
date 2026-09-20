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
	ID            string  `gorm:"primaryKey" json:"id"`
	TenantID      string  `gorm:"index;not null" json:"tenantId"`
	Name          string  `gorm:"not null" json:"name"`
	Email         string  `gorm:"uniqueIndex;not null" json:"email"`
	Role          string  `gorm:"not null;default:student" json:"role"` // student | teacher | admin
	Provider      string  `gorm:"not null;default:local" json:"provider"`
	EntraOID      *string `gorm:"column:entra_oid;uniqueIndex" json:"-"`
	AvatarDataURL string  `json:"avatarDataUrl"`
	PasswordHash  string  `gorm:"not null" json:"-"`
	// PasswordSetAt marks a password the user chose themselves (profile set /
	// change). Local accounts implicitly have one from registration; Entra
	// accounts start with an unguessable random hash and only become
	// email-loginable after they set a password.
	PasswordSetAt *time.Time     `json:"-"`
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
	Topics    []Topic   `json:"topics"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TopicCounts is per-user metadata for a topic: how many concepts it holds and
// how the current user has assessed them. The flashcard setup screen totals
// these locally instead of calling a count endpoint per scope change.
type TopicCounts struct {
	Total      int64 `json:"total"`
	Proficient int64 `json:"proficient"`
	Fuzzy      int64 `json:"fuzzy"`
	Unknown    int64 `json:"unknown"`
}

type Topic struct {
	ID        string       `gorm:"primaryKey" json:"id"`
	UnitID    string       `gorm:"index;not null" json:"unitId"`
	Title     string       `gorm:"not null" json:"title"`
	Position  int          `json:"position"`
	Counts    *TopicCounts `gorm:"-" json:"counts,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
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
	// Unit/Topic are pointers so the slim concept list (no relation preloads)
	// omits them entirely; struct fields would serialize zero-value objects.
	Unit      *Unit     `json:"unit,omitempty"`
	Topic     *Topic    `json:"topic,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
	Starred         bool       `gorm:"not null;default:false" json:"starred"`
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

// Question bank. MCQ questions carry choices + answer key + explanation;
// subjective questions (FRQ/AAQ/EBQ style) carry materials, per-part prompts
// with reference answers and rubrics for self-assessment (and a future AI
// grading workflow).
type Tag struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Stimulus is a reading shared by several questions: the passage of an MCQ
// question set, the single AAQ article, or the EBQ's three sources. Document
// bodies are markdown and may embed uploaded images (![](url)).
type Stimulus struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"index;not null" json:"title"`
	Kind      string         `gorm:"not null;default:passage" json:"kind"` // passage | article | sources
	Documents datatypes.JSON `json:"documents"`                            // []StimulusDocument
	CreatedBy string         `json:"createdBy"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type Question struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	Type        string         `gorm:"index;not null" json:"type"` // mcq | subjective
	Format      string         `gorm:"index;not null;default:''" json:"format"`
	Stem        string         `gorm:"not null" json:"stem"`
	StimulusID  *string        `gorm:"index" json:"stimulusId"`
	Materials   datatypes.JSON `json:"materials"`   // []QuestionMaterial
	Choices     datatypes.JSON `json:"choices"`     // []QuestionChoice (mcq)
	AnswerKey   string         `json:"answerKey"`   // mcq choice key, hidden from students
	Explanation string         `json:"explanation"` // hidden from students
	Parts       datatypes.JSON `json:"parts"`       // []QuestionPart (subjective)
	UnitID      *string        `gorm:"index" json:"unitId"`
	TopicID     *string        `gorm:"index" json:"topicId"`
	Status      string         `gorm:"index;not null;default:draft" json:"status"` // draft | published | archived
	Source      string         `gorm:"not null;default:manual" json:"source"`      // manual | csv | json
	SourceNote  string         `json:"sourceNote"`
	CreatedBy   string         `gorm:"index" json:"createdBy"`
	Tags        []Tag          `gorm:"many2many:question_tags" json:"tags"`
	// Concepts carries only ids in payloads; rows are attached separately as
	// lite {id, term} chips so full concept content never rides along.
	Concepts  []Concept `gorm:"many2many:question_concepts" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// QuestionFormatAAQ/EBQ mark College Board free-response layouts: AAQ answers
// per part next to the article, EBQ scaffolds claim/evidence/explanation
// parts next to three sources. FRQ (and empty, for pre-format rows) is the
// plain essay prompt.
const (
	QuestionFormatFRQ = "frq"
	QuestionFormatAAQ = "aaq"
	QuestionFormatEBQ = "ebq"
)

func validQuestionFormat(format string) bool {
	return format == "" || format == QuestionFormatFRQ || format == QuestionFormatAAQ || format == QuestionFormatEBQ
}

// PracticeSet is a curated paper assembled from the question bank. Instant
// mode gives per-question feedback; exam mode withholds feedback and grading
// until the attempt is finished (optionally under a time limit).
type PracticeSet struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Title        string    `gorm:"not null" json:"title"`
	Description  string    `json:"description"`
	Mode         string    `gorm:"not null;default:instant" json:"mode"` // instant | exam
	TimeLimitSec *int      `json:"timeLimitSec"`
	Status       string    `gorm:"index;not null;default:draft" json:"status"` // draft | published | archived
	CreatedBy    string    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type PracticeSetItem struct {
	ID         string `gorm:"primaryKey" json:"id"`
	SetID      string `gorm:"uniqueIndex:idx_set_question;not null" json:"setId"`
	QuestionID string `gorm:"uniqueIndex:idx_set_question;not null" json:"questionId"`
	Position   int    `json:"position"`
}

type PracticeAttempt struct {
	ID         string     `gorm:"primaryKey" json:"id"`
	UserID     string     `gorm:"index;not null" json:"userId"`
	SetID      string     `gorm:"index;not null" json:"setId"`
	Mode       string     `gorm:"not null" json:"mode"` // snapshot of set mode at start
	StartedAt  time.Time  `json:"startedAt"`
	DeadlineAt *time.Time `json:"deadlineAt"` // exam mode server-side deadline
	FinishedAt *time.Time `json:"finishedAt"`
	Score      *int       `json:"score"` // MCQ correct count
	TotalMCQ   int        `json:"totalMcq"`
}

// AnswerPart is one per-part response of an AAQ/EBQ question; labels match
// the question's own part labels.
type AnswerPart struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// AnswerPartRating is the per-part self-assessment (proficient | partial |
// weak). Their aggregate is always mirrored into PracticeAnswer.SelfRating.
type AnswerPartRating struct {
	Label  string `json:"label"`
	Rating string `json:"rating"`
}

type PracticeAnswer struct {
	ID         string `gorm:"primaryKey" json:"id"`
	AttemptID  string `gorm:"uniqueIndex:idx_attempt_question;not null" json:"attemptId"`
	QuestionID string `gorm:"uniqueIndex:idx_attempt_question;index:idx_practice_answer_question;not null" json:"questionId"`
	ChoiceKey  string `json:"choiceKey"`  // mcq
	TextAnswer string `json:"textAnswer"` // subjective (frq single-essay)
	// Parts/PartRatings hold per-part responses and self-ratings for AAQ/EBQ
	// questions ([]AnswerPart / []AnswerPartRating). PartRatings always keep a
	// derived aggregate in SelfRating so the wrong book keeps working.
	Parts       datatypes.JSON `json:"parts"`
	PartRatings datatypes.JSON `json:"partRatings"`
	IsCorrect   *bool          `json:"isCorrect"`
	SelfRating  string         `json:"selfRating"` // proficient | partial | weak (subjective)
	AnsweredAt  time.Time      `json:"answeredAt"`
	CreatedAt   time.Time      `json:"createdAt"`
}

// NoteResource is one tab on the Notes page: a curated bundle of study
// materials (embedded pages such as Mubu outlines, PDFs, images, plain links).
type NoteResource struct {
	ID          string             `gorm:"primaryKey" json:"id"`
	Title       string             `gorm:"not null" json:"title"`
	Description string             `json:"description"`
	Status      string             `gorm:"index;not null;default:draft" json:"status"` // draft | published | archived
	Position    int                `json:"position"`
	Items       []NoteResourceItem `gorm:"foreignKey:ResourceID" json:"items"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

// NoteResourceItem is one entry inside a Notes tab, e.g. "Unit 0" pointing at
// a Mubu share link or an internal /files PDF. Kind picks the renderer.
type NoteResourceItem struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	ResourceID string    `gorm:"index;not null" json:"resourceId"`
	Label      string    `gorm:"not null" json:"label"`
	URL        string    `gorm:"not null" json:"url"`
	Kind       string    `gorm:"not null;default:embed" json:"kind"` // embed | pdf | image | link
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Announcement is a dashboard notice posted by staff. Body uses the frontend
// inline-markdown dialect, including ==colored marker== highlights.
type Announcement struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	Title      string    `gorm:"not null" json:"title"`
	Body       string    `gorm:"not null;default:''" json:"body"`
	Pinned     bool      `gorm:"not null;default:false" json:"pinned"`
	Status     string    `gorm:"index;not null;default:draft" json:"status"` // draft | published | archived
	CreatedBy  string    `gorm:"index" json:"createdBy"`
	AuthorName string    `json:"authorName"` // snapshot of the author's display name
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// CalendarEvent is a teacher-managed deadline on the student dashboard
// calendar. Dates are local "YYYY-MM-DD" strings so timezone conversion can
// never shift the day the teacher picked; Time is a nil-able "HH:MM" (nil =
// all day). A nil EndDate means the event occupies only its start day.
type CalendarEvent struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"not null" json:"title"`
	Date      string    `gorm:"index;not null" json:"date"`
	EndDate   *string   `gorm:"index" json:"endDate,omitempty"`
	Time      *string   `json:"time,omitempty"`
	Kind      string    `gorm:"index;not null;default:event" json:"kind"` // assignment | quiz | unit-test | exam | holiday | event
	Note      string    `json:"note"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Quote is one bilingual "psychology thought of the day" shown beside the
// mascot on the dashboard; the pick rotates deterministically by date.
type Quote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TextZh    string    `gorm:"not null" json:"textZh"`
	TextEn    string    `gorm:"not null" json:"textEn"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}
