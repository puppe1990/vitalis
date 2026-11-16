package photos

import (
	"io"
	"mime/multipart"
)

// MaxRequestBytes is the ceiling for one multipart POST that may carry up to
// MaxPhotosPerMeal files, each MaxBytes, plus a little form overhead.
func MaxRequestBytes() int64 {
	return int64(MaxBytes)*int64(MaxPhotosPerMeal) + 1<<20
}

// ReadFileHeaders reads every uploaded part, capping each file at MaxBytes+1
// so Sniff can still return ErrTooLarge.
func ReadFileHeaders(headers []*multipart.FileHeader) ([][]byte, error) {
	out := make([][]byte, 0, len(headers))
	for _, hdr := range headers {
		f, err := hdr.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, body)
	}
	return out, nil
}
