package httputil

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

// Tiered buffer pool sizes.
const (
	poolSmallCap  = 64 << 10  // 64KB
	poolMediumCap = 1 << 20   // 1MB
	poolLargeCap  = 10 << 20  // 10MB
	poolXLargeCap = 50 << 20  // 50MB
	poolMaxCap    = 50 << 20  // discard buffers larger than this
)

var (
	poolSmall = sync.Pool{New: func() any {
		b := bytes.NewBuffer(make([]byte, 0, poolSmallCap))
		return b
	}}
	poolMedium = sync.Pool{New: func() any {
		b := bytes.NewBuffer(make([]byte, 0, poolMediumCap))
		return b
	}}
	poolLarge = sync.Pool{New: func() any {
		b := bytes.NewBuffer(make([]byte, 0, poolLargeCap))
		return b
	}}
	poolXLarge = sync.Pool{New: func() any {
		b := bytes.NewBuffer(make([]byte, 0, poolXLargeCap))
		return b
	}}
)

func acquireBuffer(sizeHint int) *bytes.Buffer {
	switch {
	case sizeHint <= poolSmallCap:
		return poolSmall.Get().(*bytes.Buffer)
	case sizeHint <= poolMediumCap:
		return poolMedium.Get().(*bytes.Buffer)
	case sizeHint <= poolLargeCap:
		return poolLarge.Get().(*bytes.Buffer)
	default:
		return poolXLarge.Get().(*bytes.Buffer)
	}
}

func releaseBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	cap := buf.Cap()
	buf.Reset()
	// Discard oversized buffers to prevent pool bloat.
	if cap > poolMaxCap {
		return
	}
	switch {
	case cap <= poolSmallCap:
		poolSmall.Put(buf)
	case cap <= poolMediumCap:
		poolMedium.Put(buf)
	case cap <= poolLargeCap:
		poolLarge.Put(buf)
	default:
		poolXLarge.Put(buf)
	}
}

// ReadRequestBodyPooled reads request body using a pooled buffer.
// The caller MUST call the returned release function when the body []byte is
// no longer referenced (typically via defer). The release function returns the
// underlying buffer to the pool for reuse.
func ReadRequestBodyPooled(req *http.Request) ([]byte, func(), error) {
	noop := func() {}
	if req == nil || req.Body == nil {
		return nil, noop, nil
	}

	sizeHint := requestBodyReadInitCap
	if req.ContentLength > 0 {
		if req.ContentLength > int64(poolXLargeCap) {
			sizeHint = poolXLargeCap
		} else {
			sizeHint = int(req.ContentLength)
		}
	}

	buf := acquireBuffer(sizeHint)
	if _, err := io.Copy(buf, req.Body); err != nil {
		releaseBuffer(buf)
		return nil, noop, err
	}

	body := buf.Bytes()
	released := false
	release := func() {
		if !released {
			released = true
			releaseBuffer(buf)
		}
	}
	return body, release, nil
}
