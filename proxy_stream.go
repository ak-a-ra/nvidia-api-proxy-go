// story: e01s04
// story: e01s06
// story: e02s01
package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const streamCopyBufferSize = 32 * 1024

var streamBufferPool = sync.Pool{
	New: func() any {
		buffer := make([]byte, streamCopyBufferSize)
		return &buffer
	},
}

type preparedResponseBody struct {
	body         io.ReadCloser
	decoded      bool
	dropEncoding bool
}

// decodedResponseBody streams a gzip-decoded upstream body.
//
// Close deliberately does not call the gzip reader's Close. That method is not
// a resource release: it forwards to the flate decompressor's Close, which
// reads the decompressor's sticky error field. A concurrent Read writes that
// same field, so calling it from the idle-timer goroutine while the request
// goroutine is blocked in Read is a data race (BUG-2026-09-30T082134). The
// reader holds no resource of its own either: closing the source is what
// releases the connection and unblocks an in-flight Read.
type decodedResponseBody struct {
	reader *gzip.Reader
	source io.ReadCloser
	once   sync.Once
}

func (body *decodedResponseBody) Read(buffer []byte) (int, error) {
	return body.reader.Read(buffer)
}

func (body *decodedResponseBody) Close() error {
	body.once.Do(func() {
		_ = body.source.Close()
	})
	return nil
}

func prepareResponseBody(
	response *http.Response,
	method string,
	idleTimeout time.Duration,
) (preparedResponseBody, error) {
	encoding := strings.TrimSpace(response.Header.Get("Content-Encoding"))
	if method == http.MethodHead || !responseCanHaveBody(response.StatusCode, method) {
		return preparedResponseBody{body: http.NoBody, dropEncoding: true}, nil
	}
	if !strings.EqualFold(encoding, "gzip") {
		return preparedResponseBody{body: response.Body}, nil
	}
	var headerTimer *time.Timer
	if idleTimeout > 0 {
		headerTimer = time.AfterFunc(idleTimeout, func() { _ = response.Body.Close() })
	}
	reader, err := gzip.NewReader(response.Body)
	if headerTimer != nil {
		headerTimer.Stop()
	}
	if err != nil {
		return preparedResponseBody{}, err
	}
	return preparedResponseBody{
		body:         &decodedResponseBody{reader: reader, source: response.Body},
		decoded:      true,
		dropEncoding: true,
	}, nil
}

func responseCanHaveBody(status int, method string) bool {
	return method != http.MethodHead && status != http.StatusNoContent && status != http.StatusResetContent &&
		status != http.StatusNotModified
}

func copyStreaming(writer http.ResponseWriter, source io.ReadCloser, idleTimeout time.Duration) error {
	flusher, canFlush := writer.(http.Flusher)
	var idleTimer *time.Timer
	if idleTimeout > 0 {
		idleTimer = time.AfterFunc(idleTimeout, func() { _ = source.Close() })
		defer idleTimer.Stop()
	}
	bufferPointer := streamBufferPool.Get().(*[]byte)
	defer streamBufferPool.Put(bufferPointer)
	buffer := *bufferPointer
	for {
		count, err := source.Read(buffer)
		if count > 0 {
			written, writeErr := writer.Write(buffer[:count])
			if writeErr != nil {
				return writeErr
			}
			if written != count {
				return io.ErrShortWrite
			}
			if canFlush {
				flusher.Flush()
			}
			if idleTimer != nil {
				idleTimer.Reset(idleTimeout)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
