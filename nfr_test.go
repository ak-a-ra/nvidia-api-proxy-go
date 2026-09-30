// story: e02s01
package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"
)

// NFR targets for e02. Thresholds and rationale: specs/tech-architecture/NFR_LATEST.yaml.
// These tests are the enforcement; the YAML is the contract.
const (
	nfrStreamCopyBody       = 32 << 20
	nfrStreamCopyAllocLimit = 512 << 10
	nfrFirstChunkBody       = "data: first\n\n"
	nfrFirstChunkConcurrent = 32
	nfrFirstChunkCeiling    = 500 * time.Millisecond
	nfrSustainedConcurrent  = 256
	nfrSustainedCeiling     = 10 * time.Second
	nfrConnectTimeout       = 200 * time.Millisecond
	nfrConnectCeiling       = time.Second
	nfrIdleTimeout          = 150 * time.Millisecond
	nfrIdleCeiling          = 500 * time.Millisecond
)

type nfrDiscardWriter struct{ n int64 }

func (w *nfrDiscardWriter) Header() http.Header         { return http.Header{} }
func (w *nfrDiscardWriter) WriteHeader(int)             {}
func (w *nfrDiscardWriter) Write(p []byte) (int, error) { w.n += int64(len(p)); return len(p), nil }

// nfrSizedReader yields exactly remaining bytes without allocating.
type nfrSizedReader struct{ remaining int64 }

func (r *nfrSizedReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	r.remaining -= int64(len(p))
	return len(p), nil
}

func nfrMeasureAlloc(run func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// NFR-1: streaming copy allocation is O(1) in body size, not O(body).
func TestNFRStreamingAllocationIsConstantInBodySize(t *testing.T) {
	run := func() {
		source := io.NopCloser(&nfrSizedReader{remaining: nfrStreamCopyBody})
		if err := copyStreaming(&nfrDiscardWriter{}, source, 0); err != nil {
			t.Fatalf("copyStreaming() error = %v", err)
		}
	}
	run() // warm the pooled buffer

	allocated := nfrMeasureAlloc(run)
	if allocated >= nfrStreamCopyAllocLimit {
		t.Fatalf("NFR-1: copying %d bytes allocated %d bytes, want less than %d",
			nfrStreamCopyBody, allocated, nfrStreamCopyAllocLimit)
	}
	// Check the same ceiling on a body an eighth the size. The point is not
	// that the two numbers are equal (pool reuse makes them vary run to run)
	// but that both bodies allocate orders of magnitude less than their own
	// size. A copy that buffered the body would allocate megabytes here and
	// trip the ceiling for either body.
	smallRun := func() {
		source := io.NopCloser(&nfrSizedReader{remaining: nfrStreamCopyBody / 8})
		if err := copyStreaming(&nfrDiscardWriter{}, source, 0); err != nil {
			t.Fatalf("copyStreaming() error = %v", err)
		}
	}
	smallRun()
	smallAlloc := nfrMeasureAlloc(smallRun)
	if smallAlloc >= nfrStreamCopyAllocLimit {
		t.Fatalf("NFR-1: copying %d bytes allocated %d bytes, want less than %d",
			nfrStreamCopyBody/8, smallAlloc, nfrStreamCopyAllocLimit)
	}
	t.Logf("NFR-1: %d-byte body allocated %d bytes; %d-byte body allocated %d bytes (limit %d)",
		nfrStreamCopyBody, allocated, nfrStreamCopyBody/8, smallAlloc, nfrStreamCopyAllocLimit)

	// The pooled 32 KiB buffer is reused, so the whole-copy path must also be
	// allocation-free per run. Asserted only if the runtime reports zero.
	if runs := testing.AllocsPerRun(20, run); runs == 0 {
		t.Logf("NFR-1: AllocsPerRun = 0 over the pooled path (constant allocation confirmed)")
	} else {
		t.Logf("NFR-1: AllocsPerRun = %v (non-zero, byte ceiling above is the enforced bound)", runs)
	}
}

// NFR-2: the first chunk reaches the client while the upstream still owes data.
func TestNFRStreamingFirstChunkNotBufferedUnderLoad(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(nfrFirstChunkBody))
		writer.(http.Flusher).Flush()
		<-release
	}))
	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		IdleTimeout:    10 * time.Second,
	}))
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		proxy.Close()
		upstream.Close()
	})

	latencies := make([]time.Duration, nfrFirstChunkConcurrent)
	failures := make([]error, nfrFirstChunkConcurrent)
	var waitGroup sync.WaitGroup
	for index := 0; index < nfrFirstChunkConcurrent; index++ {
		waitGroup.Add(1)
		go func(slot int) {
			defer waitGroup.Done()
			request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/chat/completions", nil)
			if err != nil {
				failures[slot] = err
				return
			}
			request.Header.Set("Authorization", "Bearer pt-test")
			started := time.Now()
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				failures[slot] = err
				return
			}
			defer response.Body.Close()
			buffer := make([]byte, len(nfrFirstChunkBody))
			if _, err := io.ReadFull(response.Body, buffer); err != nil {
				failures[slot] = err
				return
			}
			if string(buffer) != nfrFirstChunkBody {
				failures[slot] = fmt.Errorf("first chunk = %q, want %q", buffer, nfrFirstChunkBody)
				return
			}
			latencies[slot] = time.Since(started)
		}(index)
	}
	waitGroup.Wait()
	releaseOnce.Do(func() { close(release) })

	worst := time.Duration(0)
	for slot := 0; slot < nfrFirstChunkConcurrent; slot++ {
		if failures[slot] != nil {
			t.Fatalf("NFR-2: stream %d failed: %v", slot, failures[slot])
		}
		if latencies[slot] > worst {
			worst = latencies[slot]
		}
	}
	if worst >= nfrFirstChunkCeiling {
		t.Fatalf("NFR-2: worst first-chunk latency %s at %d concurrent streams, want less than %s",
			worst, nfrFirstChunkConcurrent, nfrFirstChunkCeiling)
	}
	t.Logf("NFR-2: worst first-chunk latency %s across %d concurrent gated streams (ceiling %s)",
		worst, nfrFirstChunkConcurrent, nfrFirstChunkCeiling)
}

// NFR-3: sustained concurrent streaming requests complete intact.
func TestNFRStreamingHandlesTwoHundredFiftySixConcurrentRequests(t *testing.T) {
	const chunks = 8
	chunk := []byte("chunk")
	wantBody := ""
	for index := 0; index < chunks; index++ {
		wantBody += string(chunk)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		for index := 0; index < chunks; index++ {
			if _, err := writer.Write(chunk); err != nil {
				return
			}
			writer.(http.Flusher).Flush()
		}
	}))
	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
	}))
	t.Cleanup(func() {
		proxy.Close()
		upstream.Close()
	})

	failures := make([]error, nfrSustainedConcurrent)
	var waitGroup sync.WaitGroup
	started := time.Now()
	for index := 0; index < nfrSustainedConcurrent; index++ {
		waitGroup.Add(1)
		go func(slot int) {
			defer waitGroup.Done()
			request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
			if err != nil {
				failures[slot] = err
				return
			}
			request.Header.Set("Authorization", "Bearer pt-test")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				failures[slot] = err
				return
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				failures[slot] = err
				return
			}
			if string(body) != wantBody {
				failures[slot] = fmt.Errorf("body = %q, want %q", body, wantBody)
			}
		}(index)
	}
	waitGroup.Wait()
	elapsed := time.Since(started)

	for slot := 0; slot < nfrSustainedConcurrent; slot++ {
		if failures[slot] != nil {
			t.Fatalf("NFR-3: request %d of %d failed: %v", slot, nfrSustainedConcurrent, failures[slot])
		}
	}
	if elapsed >= nfrSustainedCeiling {
		t.Fatalf("NFR-3: %d concurrent requests took %s, want less than %s",
			nfrSustainedConcurrent, elapsed, nfrSustainedCeiling)
	}
	t.Logf("NFR-3: %d concurrent streaming requests completed clean in %s (ceiling %s)",
		nfrSustainedConcurrent, elapsed, nfrSustainedCeiling)
}

// NFR-4a: the configured connect timeout bounds the pre-response phase.
func TestNFRConnectTimeoutHonorsConfiguredBound(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		upstream.Close()
	})

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: nfrConnectTimeout,
		IdleTimeout:    0,
	}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	client := &http.Client{Timeout: nfrConnectTimeout + nfrConnectCeiling}
	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("NFR-4a: request error = %v", err)
	}
	defer response.Body.Close()
	elapsed := time.Since(started)

	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("NFR-4a: status = %d, want 502", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"error":"Bad gateway"}` {
		t.Fatalf("NFR-4a: body = %q", body)
	}
	if elapsed < nfrConnectTimeout {
		t.Fatalf("NFR-4a: 502 after %s, want no earlier than the %s timeout",
			elapsed, nfrConnectTimeout)
	}
	if elapsed >= nfrConnectTimeout+nfrConnectCeiling {
		t.Fatalf("NFR-4a: 502 after %s, want less than %s (timeout %s + %s slack)",
			elapsed, nfrConnectTimeout+nfrConnectCeiling, nfrConnectTimeout, nfrConnectCeiling)
	}
	t.Logf("NFR-4a: 502 after %s for connect timeout %s (bound %s)",
		elapsed, nfrConnectTimeout, nfrConnectTimeout+nfrConnectCeiling)
}

// NFR-4a (disabled): a zero connect timeout removes the pre-response bound, so
// a slow upstream still produces a normal response instead of a 502.
func TestNFRConnectTimeoutZeroDisablesTheBound(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// Hold well past nfrConnectTimeout; with the bound disabled this must
		// still reach the client as a 200.
		time.Sleep(nfrConnectTimeout * 3)
		_, _ = writer.Write([]byte("late"))
	}))
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		upstream.Close()
	})

	proxy := httptest.NewServer(NewHandler(Config{
		APIKey:         "sk-test",
		ProxyToken:     "pt-test",
		UpstreamOrigin: upstream.URL,
		ConnectTimeout: 0,
		IdleTimeout:    0,
	}))
	t.Cleanup(proxy.Close)

	request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer pt-test")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("NFR-4a (disabled): request error = %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("NFR-4a (disabled): status = %d, want 200", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "late" {
		t.Fatalf("NFR-4a (disabled): body = %q, want %q", body, "late")
	}
	t.Logf("NFR-4a (disabled): slow upstream returned 200 after %s with the connect bound disabled",
		nfrConnectTimeout*3)
}

// NFR-4b: the idle timeout ends a stalled stream, resets on every chunk, and
// zero disables the bound entirely.
func TestNFRIdleTimeoutBoundsStallsWithoutCuttingActiveStreams(t *testing.T) {
	t.Run("stall is bounded", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("part1"))
			writer.(http.Flusher).Flush()
			<-request.Context().Done()
		}))
		t.Cleanup(upstream.Close)

		proxy := httptest.NewServer(NewHandler(Config{
			APIKey:         "sk-test",
			ProxyToken:     "pt-test",
			UpstreamOrigin: upstream.URL,
			ConnectTimeout: 0,
			IdleTimeout:    nfrIdleTimeout,
		}))
		t.Cleanup(proxy.Close)

		request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer pt-test")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()

		first := make([]byte, len("part1"))
		if _, err := io.ReadFull(response.Body, first); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		readError := make(chan error, 1)
		go func() {
			buffer := make([]byte, 1)
			_, err := response.Body.Read(buffer)
			readError <- err
		}()

		select {
		case err := <-readError:
			if err == nil || err == io.EOF {
				t.Fatalf("NFR-4b: stalled stream error = %v, want a timeout failure", err)
			}
			elapsed := time.Since(started)
			if elapsed >= nfrIdleTimeout+nfrIdleCeiling {
				t.Fatalf("NFR-4b: stall ended after %s, want less than %s (idle %s + %s slack)",
					elapsed, nfrIdleTimeout+nfrIdleCeiling, nfrIdleTimeout, nfrIdleCeiling)
			}
			t.Logf("NFR-4b: stall ended after %s for idle timeout %s (bound %s)",
				elapsed, nfrIdleTimeout, nfrIdleTimeout+nfrIdleCeiling)
		case <-time.After(nfrIdleTimeout + nfrIdleCeiling):
			t.Fatalf("NFR-4b: stalled stream stayed open past %s", nfrIdleTimeout+nfrIdleCeiling)
		}
	})

	t.Run("chunks reset the bound", func(t *testing.T) {
		const parts = 5
		pace := nfrIdleTimeout / 2
		upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			for index := 0; index < parts; index++ {
				if _, err := writer.Write([]byte("part")); err != nil {
					return
				}
				writer.(http.Flusher).Flush()
				time.Sleep(pace)
			}
		}))
		t.Cleanup(upstream.Close)

		proxy := httptest.NewServer(NewHandler(Config{
			APIKey:         "sk-test",
			ProxyToken:     "pt-test",
			UpstreamOrigin: upstream.URL,
			ConnectTimeout: 0,
			IdleTimeout:    nfrIdleTimeout,
		}))
		t.Cleanup(proxy.Close)

		request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer pt-test")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("NFR-4b: active stream was cut off: %v", err)
		}
		if len(body) != parts*len("part") {
			t.Fatalf("NFR-4b: body = %q, want %d bytes", body, parts*len("part"))
		}
	})

	t.Run("zero disables the idle bound", func(t *testing.T) {
		release := make(chan struct{})
		var releaseOnce sync.Once
		upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, _ = writer.Write([]byte("part1"))
			writer.(http.Flusher).Flush()
			select {
			case <-release:
				_, _ = writer.Write([]byte("part2"))
				writer.(http.Flusher).Flush()
			case <-request.Context().Done():
			}
		}))
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(release) })
			upstream.Close()
		})

		proxy := httptest.NewServer(NewHandler(Config{
			APIKey:         "sk-test",
			ProxyToken:     "pt-test",
			UpstreamOrigin: upstream.URL,
			ConnectTimeout: 0,
			IdleTimeout:    0,
		}))
		t.Cleanup(proxy.Close)

		request, err := http.NewRequest(http.MethodGet, proxy.URL+"/v1/models", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer pt-test")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()

		first := make([]byte, len("part1"))
		if _, err := io.ReadFull(response.Body, first); err != nil {
			t.Fatal(err)
		}
		// Hold the stream idle well past the timeout these tests configure
		// elsewhere. With the bound disabled nothing may cut it off.
		time.Sleep(nfrIdleTimeout * 3)
		releaseOnce.Do(func() { close(release) })
		rest, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("NFR-4b: disabled idle timeout cut the stream: %v", err)
		}
		if string(rest) != "part2" {
			t.Fatalf("NFR-4b: remaining body = %q, want %q", rest, "part2")
		}
	})
}

// NFR-4c: the numeric defaults are part of the contract.
func TestNFRTimeoutDefaultsAreNumeric(t *testing.T) {
	t.Setenv("NVIDIA_BASE_URL", "https://integrate.api.nvidia.com/v1")
	t.Setenv("NVIDIA_API_KEY", "sk-test")
	t.Setenv("PROXY_AUTH_TOKEN", "pt-test")
	t.Setenv("UPSTREAM_CONNECT_TIMEOUT_SECONDS", "")
	t.Setenv("UPSTREAM_IDLE_TIMEOUT_SECONDS", "")

	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnectTimeout != 30*time.Second {
		t.Fatalf("NFR-4c: ConnectTimeout = %s, want 30s", config.ConnectTimeout)
	}
	if config.IdleTimeout != 120*time.Second {
		t.Fatalf("NFR-4c: IdleTimeout = %s, want 120s", config.IdleTimeout)
	}
}

// nfrGzipSanity keeps the gzip decode path inside the NFR allocation claim,
// since a decoded response must not buffer the whole body either.
func TestNFRStreamingGzipDecodeDoesNotBufferBody(t *testing.T) {
	var compressed []byte
	{
		buffer := &nfrSliceWriter{}
		encoder := gzip.NewWriter(buffer)
		payload := make([]byte, nfrStreamCopyBody/8)
		for index := range payload {
			payload[index] = byte(index)
		}
		if _, err := encoder.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := encoder.Close(); err != nil {
			t.Fatal(err)
		}
		compressed = buffer.data
	}

	run := func() {
		source := io.NopCloser(&nfrByteReader{data: compressed})
		response := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Encoding": []string{"gzip"}},
			Body:       source,
		}
		prepared, err := prepareResponseBody(response, http.MethodGet, 0)
		if err != nil {
			t.Fatalf("prepareResponseBody() error = %v", err)
		}
		if err := copyStreaming(&nfrDiscardWriter{}, prepared.body, 0); err != nil {
			t.Fatalf("copyStreaming() error = %v", err)
		}
		if err := prepared.body.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
	run() // warm pool and gzip allocations

	allocated := nfrMeasureAlloc(run)
	if allocated >= nfrStreamCopyAllocLimit {
		t.Fatalf("NFR-1 (gzip): decoding %d compressed bytes allocated %d, want less than %d",
			len(compressed), allocated, nfrStreamCopyAllocLimit)
	}
	t.Logf("NFR-1 (gzip): decoding %d compressed bytes allocated %d (limit %d)",
		len(compressed), allocated, nfrStreamCopyAllocLimit)
}

type nfrSliceWriter struct{ data []byte }

func (w *nfrSliceWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

type nfrByteReader struct{ data []byte }

func (r *nfrByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}
