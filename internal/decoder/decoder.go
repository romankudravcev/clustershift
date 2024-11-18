package decoder

import "encoding/base64"

func DecodeBase64String(encoded string) string {
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(decoded)
}
