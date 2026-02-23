package qrcode

import (
	"encoding/base64"
	"fmt"

	"github.com/skip2/go-qrcode"
)

// GenerateQRCode generates a QR code and returns it as base64 string
func GenerateQRCode(content string) (string, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %v", err)
	}

	base64Str := base64.StdEncoding.EncodeToString(png)
	return "data:image/png;base64," + base64Str, nil
}

// GenerateQRCodeBytes generates a QR code and returns the PNG bytes
func GenerateQRCodeBytes(content string) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, 256)
}
