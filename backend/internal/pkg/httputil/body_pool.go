package httputil

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

// Tiered buffer pool sizes.
const (
	poolSmallCap  = 64 << 10 // 64KB
	poolMediumCap = 1 << 20  // 1MB
	poolLargeCap  = 10 << 20 // 10MB
	poolXLargeCap = 50 << 20 // 50MB
	poolMaxCap    = 50 << 20 // discard buffers larger than this
)

var (
	poolSmall = sync.Pool{New: func() any {
		return bytes.NewBuffer(make([]byte, 0, poolSmallCap))
	}}
	poolMedium = sync.Pool{New: func() any {
		return bytes.NewBuffer(make([]byte, 0, poolMediumCap))
	}}
	poolLarge = sync.Pool{New: func() any {
		return bytes.NewBuffer(make([]byte, 0, poolLargeCap))
	}}
	poolXLarge = sync.Pool{New: func() any {
		return bytes.NewBuffer(make([]byte, 0, poolXLargeCap))
	}}
)

func acquireBuffer(sizeHint int) *bytes.Buffer {
	var v any
	switch {
	case sizeHint <= poolSmallCap:
		v = poolSmall.Get()
	case sizeHint <= poolMediumCap:
		v = poolMedium.Get()
	case sizeHint <= poolLargeCap:
		v = poolLarge.Get()
	default:
		v = poolXLarge.Get()
	}
	buf, ok := v.(*bytes.Buffer)
	if !ok {
		return bytes.NewBuffer(make([]byte, 0, sizeHint))
	}
	return buf
}

func releaseBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	c := buf.Cap()
	buf.Reset()
	// Discard oversized buffers to prevent pool bloat.
	if c > poolMaxCap {
		return
	}
	switch {
	case c <= poolSmallCap:
		poolSmall.Put(buf)
	case c <= poolMediumCap:
		poolMedium.Put(buf)
	case c <= poolLargeCap:
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
