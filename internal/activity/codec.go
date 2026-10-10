package activity

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// CompressThresholdBytes mirrors the agreed design: payloads below this stay
// plain TEXT (directly inspectable in SQL); at/above it they are gzipped.
const CompressThresholdBytes = 1024

// Encode compresses payload when it reaches the threshold and reports the
// codec to persist alongside the blob. Small payloads stay plain for
// debuggability. Empty payload returns nil blob with CodecNone.
func Encode(payload []byte) (blob []byte, codec string, err error) {
	if len(payload) == 0 {
		return nil, CodecNone, nil
	}
	if len(payload) < CompressThresholdBytes {
		return payload, CodecNone, nil
	}
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return nil, "", fmt.Errorf("gzip writer: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return nil, "", fmt.Errorf("gzip write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), CodecGzip, nil
}

// EncodeForced encodes with an explicit codec instead of the size threshold.
// It exists so ledger rows keep one codec for both snapshots.
func EncodeForced(payload []byte, codec string) ([]byte, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	switch codec {
	case "", CodecNone:
		return payload, nil
	case CodecGzip:
		var buf bytes.Buffer
		w, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
		if err != nil {
			return nil, fmt.Errorf("gzip writer: %w", err)
		}
		if _, err := w.Write(payload); err != nil {
			return nil, fmt.Errorf("gzip write: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("gzip close: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported activity payload codec %q", codec)
	}
}

// Decode reverses Encode. Unknown codecs are rejected Fail-Closed instead of
// being returned as garbled text.
func Decode(blob []byte, codec string) ([]byte, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	switch codec {
	case "", CodecNone:
		return blob, nil
	case CodecGzip:
		r, err := gzip.NewReader(bytes.NewReader(blob))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		out, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("gzip read: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported activity payload codec %q", codec)
	}
}
