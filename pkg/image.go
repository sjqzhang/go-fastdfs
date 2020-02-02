package pkg

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"

	"github.com/nfnt/resize"
	log "github.com/sirupsen/logrus"
)

// ResizeImage
func ResizeImage(w http.ResponseWriter, fullPath string, width, height uint) {
	file, err := os.Open(fullPath)
	if err != nil {
		log.Error(err)
		return
	}

	img, imgType, err := image.Decode(file)
	if err != nil {
		log.Error(err)
		return
	}
	file.Close()

	img = resize.Resize(width, height, img, resize.Lanczos3)
	switch imgType {
	case "jpg", "jpeg":
		jpeg.Encode(w, img, nil)
	case "png":
		png.Encode(w, img)
	default:
		file.Seek(0, 0)
		io.Copy(w, file)
	}
}

// ResizeImageByBytes
func ResizeImageByBytes(w http.ResponseWriter, data []byte, width, height uint) {
	var (
		img     image.Image
		err     error
		imgType string
	)
	reader := bytes.NewReader(data)
	img, imgType, err = image.Decode(reader)
	if err != nil {
		log.Error(err)
		return
	}

	img = resize.Resize(width, height, img, resize.Lanczos3)
	switch imgType {
	case "jpg", "jpeg":
		jpeg.Encode(w, img, nil)
	case "png":
		png.Encode(w, img)
	default:
		w.Write(data)
	}
}
