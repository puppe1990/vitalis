package photos

import (
	"bytes"
	"errors"
	"fmt"
)

// MaxBytes is the largest plate photo the diary accepts. iPhone HEIC plates
// land around 2–4 MB; 8 MB leaves room without turning the POST into a dump.
const MaxBytes = 8 << 20

var (
	ErrUnsupportedType = errors.New("unsupported image type")
	ErrTooLarge        = errors.New("image exceeds 8 MB")
)

// Kind is the sniffed image type. The extension is what we put on the B2 key
// so a downloaded file opens in Photos without a rename.
type Kind struct {
	ContentType string
	Ext         string
}

// Sniff reads magic bytes. The filename is ignored: a .jpg wrapping a PDF
// must not land in the bucket.
func Sniff(body []byte) (Kind, error) {
	if len(body) == 0 {
		return Kind{}, fmt.Errorf("%w: empty body", ErrUnsupportedType)
	}
	if len(body) > MaxBytes {
		return Kind{}, fmt.Errorf("%w: %d bytes", ErrTooLarge, len(body))
	}
	switch {
	case bytes.HasPrefix(body, []byte{0xff, 0xd8, 0xff}):
		return Kind{ContentType: "image/jpeg", Ext: "jpg"}, nil
	case bytes.HasPrefix(body, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}):
		return Kind{ContentType: "image/png", Ext: "png"}, nil
	case isWebP(body):
		return Kind{ContentType: "image/webp", Ext: "webp"}, nil
	case isHEIF(body):
		return Kind{ContentType: "image/heic", Ext: "heic"}, nil
	default:
		return Kind{}, fmt.Errorf("%w", ErrUnsupportedType)
	}
}

func isWebP(body []byte) bool {
	return len(body) >= 12 &&
		bytes.Equal(body[0:4], []byte("RIFF")) &&
		bytes.Equal(body[8:12], []byte("WEBP"))
}

func isHEIF(body []byte) bool {
	// ISO BMFF: 4-byte box size, then "ftyp", then a 4-byte brand. iPhone
	// photos use heic/heif/mif1/msf1 depending on the codec.
	if len(body) < 12 || !bytes.Equal(body[4:8], []byte("ftyp")) {
		return false
	}
	brand := string(body[8:12])
	switch brand {
	case "heic", "heif", "heix", "hevc", "heim", "mif1", "msf1":
		return true
	}
	return false
}
