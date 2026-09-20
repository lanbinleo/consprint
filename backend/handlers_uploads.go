package backend

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// Shared multipart upload core: both note attachments and question images
// flow through storeMultipart; only the size cap, allowed types (by sniffed
// content, never by client filename), and naming differ.

type uploadSpec struct {
	MaxBytes int64
	Exts     map[string]string
	TooLarge string
	BadType  string
	Name     func(header *multipart.FileHeader, ext string) string
}

// storeMultipart validates and stores the "file" form field under PublicDir
// and returns the stored filename. JSON error responses are already written
// when ok is false.
func (a *App) storeMultipart(c *gin.Context, spec uploadSpec) (string, bool) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "file is required"})
		return "", false
	}
	defer file.Close()
	if header.Size > spec.MaxBytes {
		c.JSON(400, gin.H{"error": spec.TooLarge})
		return "", false
	}
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		c.JSON(400, gin.H{"error": "could not read file"})
		return "", false
	}
	ext, ok := spec.Exts[http.DetectContentType(head[:n])]
	if !ok {
		c.JSON(400, gin.H{"error": spec.BadType})
		return "", false
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		c.JSON(500, gin.H{"error": "could not read file"})
		return "", false
	}
	name := spec.Name(header, ext)
	if err := os.MkdirAll(a.PublicDir, 0o755); err != nil {
		c.JSON(500, gin.H{"error": "could not store file"})
		return "", false
	}
	dst, err := os.Create(filepath.Join(a.PublicDir, name))
	if err != nil {
		c.JSON(500, gin.H{"error": "could not store file"})
		return "", false
	}
	defer dst.Close()
	written, err := io.Copy(dst, file)
	if err != nil || written == 0 {
		c.JSON(500, gin.H{"error": "could not store file"})
		return "", false
	}
	return name, true
}

const maxQuestionImageBytes = 5 << 20

// questionImageExts limits question images to browser-renderable raster
// formats; the stored extension always comes from the sniffed type.
var questionImageExts = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// uploadQuestionImage stores one image for question/stimulus bodies and
// returns its /files URL to embed as markdown. Names carry random hex:
// /files is unauthenticated (img tags cannot send the Bearer header), so
// URLs must stay unguessable to keep unreleased exam figures private.
func (a *App) uploadQuestionImage(c *gin.Context) {
	name, ok := a.storeMultipart(c, uploadSpec{
		MaxBytes: maxQuestionImageBytes,
		Exts:     questionImageExts,
		TooLarge: "image exceeds the 5MB limit",
		BadType:  "only png, jpg, gif, and webp images are supported",
		Name: func(header *multipart.FileHeader, ext string) string {
			return questionImageName(ext)
		},
	})
	if !ok {
		return
	}
	c.JSON(200, gin.H{"url": "/files/" + name})
}

// questionImageName builds the unguessable stored filename for a question
// image (shared by the upload endpoint and base64 localization).
func questionImageName(ext string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("qimg-%d-%s%s", time.Now().Unix(), hex.EncodeToString(b[:]), ext)
}

// storeImageBytes writes raw image bytes (already size/type-checked) under
// PublicDir and returns the stored filename.
func (a *App) storeImageBytes(raw []byte, ext string) (string, bool) {
	if len(raw) == 0 || len(raw) > maxQuestionImageBytes {
		return "", false
	}
	name := questionImageName(ext)
	if err := os.MkdirAll(a.PublicDir, 0o755); err != nil {
		return "", false
	}
	if err := os.WriteFile(filepath.Join(a.PublicDir, name), raw, 0o644); err != nil {
		return "", false
	}
	return name, true
}
