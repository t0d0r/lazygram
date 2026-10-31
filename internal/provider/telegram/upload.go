package telegram

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

// uploadFunc sends data to a peer as a photo or a file, with a caption.
type uploadFunc func(ctx context.Context, peer tg.InputPeerClass, name string, data []byte, caption string, photo bool) error

// maxUpload caps what lazygram sends from the terminal.
const maxUpload = 64 << 20

// uploadWith builds the real uploader: upload.saveFilePart in four
// threads, then messages.sendMedia through gotd's message sender.
func uploadWith(raw *tg.Client) uploadFunc {
	return func(ctx context.Context, peer tg.InputPeerClass, name string, data []byte, caption string, photo bool) error {
		if len(data) == 0 {
			return errors.New("telegram: empty file")
		}
		if len(data) > maxUpload {
			return fmt.Errorf("telegram: file larger than %d MiB", maxUpload>>20)
		}
		f, err := uploader.NewUploader(raw).WithThreads(4).FromBytes(ctx, filepath.Base(name), data)
		if err != nil {
			return fmt.Errorf("telegram: upload: %w", err)
		}
		var text []styling.StyledTextOption
		if caption != "" {
			text = append(text, styling.Plain(caption))
		}
		to := message.NewSender(raw).To(peer)
		if photo {
			_, err = to.Media(ctx, message.UploadedPhoto(f, text...))
		} else {
			mt := mime.TypeByExtension(filepath.Ext(name))
			if mt == "" {
				mt = "application/octet-stream"
			}
			_, err = to.Media(ctx, message.UploadedDocument(f, text...).Filename(filepath.Base(name)).MIME(mt))
		}
		if err != nil {
			return fmt.Errorf("telegram: send file: %w", err)
		}
		return nil
	}
}
