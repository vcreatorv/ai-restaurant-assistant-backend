package http

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/example/ai-restaurant-assistant-backend/internal/menu"
)

// extByMime разрешённые mime-types и соответствующие расширения
var extByMime = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// readImagePart выбирает part с form-name=file, валидирует mime и размер,
// читает содержимое в память и возвращает источник для usecase
func readImagePart(reader *multipart.Reader, maxSize int64) (menu.DishImageSource, func(), error) {
	noop := func() {}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return menu.DishImageSource{}, noop, errors.New("no file part in form")
		}
		if err != nil {
			return menu.DishImageSource{}, noop, err
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		ct := part.Header.Get("Content-Type")
		ext, ok := extByMime[ct]
		if !ok {
			_ = part.Close()
			return menu.DishImageSource{}, noop, menu.ErrImageUnsupportedType
		}
		if e := strings.TrimPrefix(filepath.Ext(part.FileName()), "."); e != "" {
			ext = strings.ToLower(e)
		}

		buf := &bytes.Buffer{}
		n, err := io.Copy(buf, io.LimitReader(part, maxSize+1))
		_ = part.Close()
		if err != nil {
			return menu.DishImageSource{}, noop, err
		}
		if n > maxSize {
			return menu.DishImageSource{}, noop, menu.ErrImageTooLarge
		}

		return menu.DishImageSource{
			Body:        buf,
			ContentType: ct,
			Size:        n,
			Ext:         ext,
		}, noop, nil
	}
}
