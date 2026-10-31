package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"

	"lazygram/internal/domain"
)

// mediaOf describes a message's attachment when lazygram can fetch it:
// photos and documents (files). Other media kinds give nil.
func mediaOf(m *tg.Message) *domain.Media {
	switch media := m.Media.(type) {
	case *tg.MessageMediaPhoto:
		if _, ok := media.Photo.(*tg.Photo); ok {
			return &domain.Media{Kind: "photo"}
		}
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return nil
		}
		kind := "file"
		if strings.HasPrefix(doc.MimeType, "image/") {
			kind = "image"
		}
		return &domain.Media{Kind: kind, Name: fileName(doc)}
	}
	return nil
}

func fileName(doc *tg.Document) string {
	for _, a := range doc.Attributes {
		if f, ok := a.(*tg.DocumentAttributeFilename); ok {
			return path.Base(strings.ReplaceAll(f.FileName, "\\", "/"))
		}
	}
	return ""
}

// Fetch downloads a message's photo or file into the media directory and
// returns its path. The message is fetched again first because file
// references expire. A file already present is returned as it is.
func (s *service) Fetch(ctx context.Context, chatID, msgID string) (string, error) {
	ent, ok := s.known[chatID]
	if !ok {
		return "", fmt.Errorf("telegram: unknown chat %s", chatID)
	}
	id, err := strconv.Atoi(msgID)
	if err != nil {
		return "", fmt.Errorf("telegram: bad message id %q", msgID)
	}
	if s.files == nil || s.mediaDir == "" {
		return "", errors.New("telegram: downloads are not available")
	}
	m, _, err := s.rawMessage(ctx, ent, id)
	if err != nil {
		return "", fmt.Errorf("telegram: fetch: %w", err)
	}
	loc, ext, err := location(m)
	if err != nil {
		return "", fmt.Errorf("telegram: %w", err)
	}
	safe := strings.NewReplacer(":", "_", "/", "_", "\\", "_").Replace(chatID)
	out := filepath.Join(s.mediaDir, fmt.Sprintf("%s-%d%s", safe, id, ext))
	if _, err := os.Stat(out); err == nil {
		return out, nil
	}
	if err := os.MkdirAll(s.mediaDir, 0o700); err != nil {
		return "", err
	}
	tmp := out + ".part"
	if _, err := downloader.NewDownloader().Download(s.files, loc).ToPath(ctx, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("telegram: download: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return "", err
	}
	return out, os.Rename(tmp, out)
}

// location picks what to download for a message: the largest photo size,
// or the document itself, with a file extension for the viewer.
func location(m *tg.Message) (tg.InputFileLocationClass, string, error) {
	switch media := m.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return nil, "", errors.New("photo is not available")
		}
		best, area := "", -1
		for _, sc := range photo.Sizes {
			var typ string
			var a int
			switch sz := sc.(type) {
			case *tg.PhotoSize:
				typ, a = sz.Type, sz.W*sz.H
			case *tg.PhotoSizeProgressive:
				typ, a = sz.Type, sz.W*sz.H
			default:
				continue
			}
			if a > area {
				best, area = typ, a
			}
		}
		if best == "" {
			return nil, "", errors.New("photo has no downloadable size")
		}
		return &tg.InputPhotoFileLocation{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference, ThumbSize: best}, ".jpg", nil
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return nil, "", errors.New("file is not available")
		}
		ext := filepath.Ext(fileName(doc))
		if ext == "" {
			ext = extFor(doc.MimeType)
		}
		return &tg.InputDocumentFileLocation{ID: doc.ID, AccessHash: doc.AccessHash, FileReference: doc.FileReference}, ext, nil
	}
	return nil, "", errors.New("message has no photo or file")
}

func extFor(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "application/pdf":
		return ".pdf"
	}
	return ".bin"
}
