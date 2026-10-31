package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
		if imageExt(doc.MimeType) != "" {
			kind = "image"
		}
		return &domain.Media{Kind: kind, Name: fileName(doc)}
	}
	return nil
}

// imageExt is the file extension for a raster image mime type that lazygram
// hands to the system viewer; "" for every other type (svg is excluded: it
// opens in a browser and can carry script). The sender's file name never
// decides the extension of a file that gets opened, because the extension
// picks the application.
func imageExt(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}

func fileName(doc *tg.Document) string {
	for _, a := range doc.Attributes {
		if f, ok := a.(*tg.DocumentAttributeFilename); ok {
			return path.Base(strings.ReplaceAll(f.FileName, "\\", "/"))
		}
	}
	return ""
}

// Fetch downloads a message's photo or file into memory and returns the
// bytes with a file name. The message is fetched again first because file
// references expire. Nothing is written to disk here.
func (s *service) Fetch(ctx context.Context, chatID, msgID string) ([]byte, string, error) {
	ent, id, err := s.locate(chatID, msgID)
	if err != nil {
		return nil, "", err
	}
	return s.download(ctx, ent, id, msgID)
}

// locate resolves a chat and message id for a download; it is the only
// part of a fetch that touches the service's shared state.
func (s *service) locate(chatID, msgID string) (entity, int, error) {
	ent, ok := s.known[chatID]
	if !ok {
		return entity{}, 0, fmt.Errorf("telegram: unknown chat %s", chatID)
	}
	id, err := strconv.Atoi(msgID)
	if err != nil {
		return entity{}, 0, fmt.Errorf("telegram: bad message id %q", msgID)
	}
	if s.files == nil {
		return entity{}, 0, errors.New("telegram: downloads are not available")
	}
	return ent, id, nil
}

// download is the network part of a fetch; it needs no lock, so other
// calls proceed while a large file streams in.
func (s *service) download(ctx context.Context, ent entity, id int, msgID string) ([]byte, string, error) {
	m, _, err := s.rawMessage(ctx, ent, id)
	if err != nil {
		return nil, "", fmt.Errorf("telegram: fetch: %w", err)
	}
	loc, ext, err := location(m)
	if err != nil {
		return nil, "", fmt.Errorf("telegram: %w", err)
	}
	var buf bytes.Buffer
	if _, err := downloader.NewDownloader().Download(s.files, loc).WithThreads(4).Stream(ctx, &limitWriter{w: &buf, left: maxMedia}); err != nil {
		return nil, "", fmt.Errorf("telegram: download: %w", err)
	}
	name := msgID + ext
	if md := mediaOf(m); md != nil && md.Name != "" && md.Kind == "file" {
		name = md.Name // plain files keep the sender's name; images never do
	}
	return buf.Bytes(), name, nil
}

// maxMedia caps a download so a huge file cannot exhaust memory.
const maxMedia = 64 << 20

type limitWriter struct {
	w    io.Writer
	left int
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if len(p) > l.left {
		return 0, fmt.Errorf("attachment larger than %d MiB", maxMedia>>20)
	}
	l.left -= len(p)
	return l.w.Write(p)
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
		ext := imageExt(doc.MimeType)
		if ext == "" { // never opened by lazygram: the sender's name is only for the user
			ext = filepath.Ext(fileName(doc))
			if ext == "" {
				ext = extFor(doc.MimeType)
			}
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
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	case "audio/ogg":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "image/heic":
		return ".heic"
	case "application/pdf":
		return ".pdf"
	}
	return ".bin"
}
