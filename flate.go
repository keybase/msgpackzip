package msgpackzip

import (
	"bytes"
	"compress/flate"
	"io"
	"math"
)

func flateCompress(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	_, err = zw.Write(b)
	if err != nil {
		return nil, err
	}
	err = zw.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// flateInflateWithLimit decompresses b, returning ErrOutputTooBig if the
// result exceeds maxSize bytes. maxSize must be positive.
func flateInflateWithLimit(b []byte, maxSize int64) ([]byte, error) {
	zr := flate.NewReader(bytes.NewBuffer(b))
	defer zr.Close() //nolint:errcheck // reader Close only returns decompressor to pool
	// Read one byte past maxSize to detect oversize; guard against overflow.
	limit := maxSize
	if limit < math.MaxInt64 {
		limit++
	}
	out, err := io.ReadAll(io.LimitReader(zr, limit))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > maxSize {
		return nil, ErrOutputTooBig
	}
	return out, nil
}
