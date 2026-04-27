package content

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

type AnalyzeResult struct {
	HashHex   string
	MIMEType  string
	SizeBytes int64
}

func AnalyzeReader(reader io.Reader) (AnalyzeResult, error) {
	hasher := sha256.New()
	sniffBuf := make([]byte, 0, 512)
	total := int64(0)

	buf := make([]byte, 64*1024)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if _, writeErr := hasher.Write(chunk); writeErr != nil {
				return AnalyzeResult{}, fmt.Errorf("hash write: %w", writeErr)
			}
			total += int64(n)
			if len(sniffBuf) < 512 {
				needed := 512 - len(sniffBuf)
				if needed > n {
					needed = n
				}
				sniffBuf = append(sniffBuf, chunk[:needed]...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return AnalyzeResult{}, fmt.Errorf("analyze reader: %w", err)
		}
	}

	mimeType := "application/octet-stream"
	if len(sniffBuf) > 0 {
		mimeType = http.DetectContentType(sniffBuf)
	}

	return AnalyzeResult{
		HashHex:   hex.EncodeToString(hasher.Sum(nil)),
		MIMEType:  mimeType,
		SizeBytes: total,
	}, nil
}
