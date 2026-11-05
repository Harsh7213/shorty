package qr

import (
	"github.com/skip2/go-qrcode"
)

func GenerateQR(url string) ([]byte, error) {
	return qrcode.Encode(url, qrcode.Medium, 256)
}
