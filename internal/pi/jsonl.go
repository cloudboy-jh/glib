package pi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const maxJSONLLineBytes = 1024 * 1024

func readJSONLRecord(r io.Reader, buf *[]byte) ([]byte, error) {
	if cap(*buf) == 0 {
		*buf = make([]byte, 0, 4096)
	}
	tmp := make([]byte, 4096)
	for {
		if i := bytes.IndexByte(*buf, '\n'); i >= 0 {
			rec := append([]byte(nil), (*buf)[:i]...)
			*buf = append((*buf)[:0], (*buf)[i+1:]...)
			rec = bytes.TrimSuffix(rec, []byte{'\r'})
			return rec, nil
		}

		n, err := r.Read(tmp)
		if n > 0 {
			*buf = append(*buf, tmp[:n]...)
			if len(*buf) > maxJSONLLineBytes {
				return nil, fmt.Errorf("jsonl record exceeds %d bytes", maxJSONLLineBytes)
			}
		}
		if err != nil {
			if err == io.EOF {
				if len(*buf) == 0 {
					return nil, io.EOF
				}
				rec := append([]byte(nil), *buf...)
				*buf = (*buf)[:0]
				rec = bytes.TrimSuffix(rec, []byte{'\r'})
				return rec, nil
			}
			return nil, err
		}
	}
}

func decodeJSONL(line []byte) (map[string]any, error) {
	line = []byte(strings.TrimSpace(string(line)))
	if len(line) == 0 {
		return map[string]any{"type": "noop"}, nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(line, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
