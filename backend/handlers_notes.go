package backend

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// seedMubuNotes lists the per-unit Mubu outline share links behind the first
// Notes tab. Mubu share pages are JS apps and commonly refuse third-party
// iframes via X-Frame-Options; the frontend falls back to an open-in-new-tab
// card when the embed does not render.
var seedMubuNotes = []struct{ Label, URL string }{
	{"Unit 0", "https://share.mubu.com/doc/3nBVNn9h_7i"},
	{"Unit 1", "https://share.mubu.com/doc/5U0P50RWrAi"},
	{"Unit 2", "https://share.mubu.com/doc/1EYzB-1O3Ai"},
	{"Unit 3", "https://share.mubu.com/doc/7NNmkiBqrAi"},
	{"Unit 4", "https://share.mubu.com/doc/74_56kzyKki"},
	{"Unit 5", "https://share.mubu.com/doc/4s7J11Y-SAi"},
}

// seedNoteResources installs the default Notes tabs on an empty table only:
// Leo's per-unit Mubu outlines plus a demo PDF tab proving the /files path.
func seedNoteResources(db *gorm.DB) error {
	var count int64
	db.Model(&NoteResource{}).Count(&count)
	if count > 0 {
		return nil
	}
	leo := NoteResource{ID: NewID("note"), Title: "Leo 的复习笔记", Description: "按单元整理的幕布大纲笔记", Status: "published", Position: 0}
	if err := db.Create(&leo).Error; err != nil {
		return err
	}
	for i, unit := range seedMubuNotes {
		item := NoteResourceItem{ID: NewID("nri"), ResourceID: leo.ID, Label: unit.Label, URL: unit.URL, Kind: "embed", Position: i}
		if err := db.Create(&item).Error; err != nil {
			return err
		}
	}
	pdfTab := NoteResource{ID: NewID("note"), Title: "示例：PDF 讲义", Description: "内部文件嵌入示例（管理员可上传 PDF/图片后经 /files 提供）", Status: "published", Position: 1}
	if err := db.Create(&pdfTab).Error; err != nil {
		return err
	}
	pdfItem := NoteResourceItem{ID: NewID("nri"), ResourceID: pdfTab.ID, Label: "样例讲义", URL: "/files/ap-psych-sample.pdf", Kind: "pdf", Position: 0}
	return db.Create(&pdfItem).Error
}

type noteItemPayload struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Kind  string `json:"kind"`
}

func validNoteItemKind(kind string) bool {
	return kind == "embed" || kind == "pdf" || kind == "image" || kind == "link"
}

// validNoteResourceURL accepts absolute http(s) URLs and same-app absolute
// paths (internal /files links). Protocol-relative "//host" is rejected.
func validNoteResourceURL(raw string) bool {
	u := strings.TrimSpace(raw)
	if u == "" || strings.HasPrefix(u, "//") {
		return false
	}
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "/")
}

func validateNoteItems(items []noteItemPayload) string {
	for i, item := range items {
		if strings.TrimSpace(item.Label) == "" {
			return fmt.Sprintf("item %d: label is required", i+1)
		}
		if !validNoteResourceURL(item.URL) {
			return fmt.Sprintf("item %d: url must start with http://, https:// or /", i+1)
		}
		if !validNoteItemKind(item.Kind) {
			return fmt.Sprintf("item %d: kind must be embed, pdf, image or link", i+1)
		}
	}
	return ""
}

// attachNoteItems loads items for the given resources in one query and pins
// them onto each resource, normalizing empty sets to [] instead of null.
func attachNoteItems(db *gorm.DB, resources []NoteResource) {
	if len(resources) == 0 {
		return
	}
	ids := make([]string, 0, len(resources))
	for _, r := range resources {
		ids = append(ids, r.ID)
	}
	var items []NoteResourceItem
	db.Where("resource_id in ?", ids).Order("position asc").Find(&items)
	grouped := make(map[string][]NoteResourceItem, len(resources))
	for _, item := range items {
		grouped[item.ResourceID] = append(grouped[item.ResourceID], item)
	}
	for i := range resources {
		if grouped[resources[i].ID] == nil {
			resources[i].Items = make([]NoteResourceItem, 0)
			continue
		}
		resources[i].Items = grouped[resources[i].ID]
	}
}

// noteResources serves the published Notes tabs to any signed-in user.
func (a *App) noteResources(c *gin.Context) {
	resources := make([]NoteResource, 0)
	a.DB.Where("status = ?", "published").Order("position asc, created_at asc").Find(&resources)
	attachNoteItems(a.DB, resources)
	c.JSON(200, resources)
}

func (a *App) listNoteResources(c *gin.Context) {
	resources := make([]NoteResource, 0)
	a.DB.Order("position asc, created_at asc").Find(&resources)
	attachNoteItems(a.DB, resources)
	c.JSON(200, resources)
}

func (a *App) createNoteResource(c *gin.Context) {
	var req struct {
		Title       string            `json:"title"`
		Description string            `json:"description"`
		Status      string            `json:"status"`
		Position    *int              `json:"position"`
		Items       []noteItemPayload `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		c.JSON(400, gin.H{"error": "title is required"})
		return
	}
	status := req.Status
	if status == "" {
		status = "draft"
	}
	if status != "draft" && status != "published" && status != "archived" {
		c.JSON(400, gin.H{"error": "invalid status"})
		return
	}
	if msg := validateNoteItems(req.Items); msg != "" {
		c.JSON(400, gin.H{"error": msg})
		return
	}
	position := 0
	if req.Position != nil {
		position = *req.Position
	} else {
		var count int64
		a.DB.Model(&NoteResource{}).Count(&count)
		position = int(count)
	}
	resource := NoteResource{ID: NewID("note"), Title: strings.TrimSpace(req.Title), Description: strings.TrimSpace(req.Description), Status: status, Position: position}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&resource).Error; err != nil {
			return err
		}
		return replaceNoteItems(tx, resource.ID, req.Items)
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not create note resource"})
		return
	}
	c.JSON(200, resource)
}

func replaceNoteItems(tx *gorm.DB, resourceID string, payloads []noteItemPayload) error {
	if err := tx.Where("resource_id = ?", resourceID).Delete(&NoteResourceItem{}).Error; err != nil {
		return err
	}
	for position, item := range payloads {
		created := NoteResourceItem{ID: NewID("nri"), ResourceID: resourceID, Label: strings.TrimSpace(item.Label), URL: strings.TrimSpace(item.URL), Kind: item.Kind, Position: position}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *App) updateNoteResource(c *gin.Context) {
	var resource NoteResource
	if err := a.DB.First(&resource, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	var req struct {
		Title       *string           `json:"title"`
		Description *string           `json:"description"`
		Status      *string           `json:"status"`
		Position    *int              `json:"position"`
		Items       []noteItemPayload `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			c.JSON(400, gin.H{"error": "title cannot be empty"})
			return
		}
		resource.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		resource.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		if *req.Status != "draft" && *req.Status != "published" && *req.Status != "archived" {
			c.JSON(400, gin.H{"error": "invalid status"})
			return
		}
		resource.Status = *req.Status
	}
	if req.Position != nil {
		resource.Position = *req.Position
	}
	if msg := validateNoteItems(req.Items); msg != "" {
		c.JSON(400, gin.H{"error": msg})
		return
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&resource).Error; err != nil {
			return err
		}
		if req.Items != nil {
			return replaceNoteItems(tx, resource.ID, req.Items)
		}
		return nil
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not save note resource"})
		return
	}
	c.JSON(200, resource)
}

// deleteNoteResource is a hard delete (the app's first): note tabs have no
// dependent records elsewhere, so cleanup removes rows rather than archiving.
func (a *App) deleteNoteResource(c *gin.Context) {
	id := c.Param("id")
	var resource NoteResource
	if err := a.DB.First(&resource, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("resource_id = ?", id).Delete(&NoteResourceItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&NoteResource{}, "id = ?", id).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"error": "could not delete note resource"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

const maxNoteUploadBytes = 25 << 20

// noteUploadExts maps sniffed content types to canonical extensions; the
// stored extension always comes from the sniffed type, never the client name.
var noteUploadExts = map[string]string{
	"application/pdf": ".pdf",
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/gif":       ".gif",
	"image/webp":      ".webp",
}

// uploadNoteResourceFile stores one PDF/image under PublicDir and returns its
// /files URL for use as an internal link on a note item.
func (a *App) uploadNoteResourceFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	if header.Size > maxNoteUploadBytes {
		c.JSON(400, gin.H{"error": "file exceeds the 25MB limit"})
		return
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		c.JSON(400, gin.H{"error": "could not read file"})
		return
	}
	ext, ok := noteUploadExts[http.DetectContentType(head[:n])]
	if !ok {
		c.JSON(400, gin.H{"error": "only PDF and image files are supported"})
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(500, gin.H{"error": "could not read file"})
		return
	}
	slug := Slugify(strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)))
	if runes := []rune(slug); len(runes) > 40 {
		slug = string(runes[:40])
	}
	name := fmt.Sprintf("notes-%d-%s%s", time.Now().Unix(), slug, ext)
	if err := os.MkdirAll(a.PublicDir, 0o755); err != nil {
		c.JSON(500, gin.H{"error": "could not store file"})
		return
	}
	dst, err := os.Create(filepath.Join(a.PublicDir, name))
	if err != nil {
		c.JSON(500, gin.H{"error": "could not store file"})
		return
	}
	defer dst.Close()
	written, err := io.Copy(dst, file)
	if err != nil || written == 0 {
		c.JSON(500, gin.H{"error": "could not store file"})
		return
	}
	c.JSON(200, gin.H{"url": "/files/" + name, "filename": name})
}
