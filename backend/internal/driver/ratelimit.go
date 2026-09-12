package driver

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

// newByteLimiter builds a token-bucket limiter for bps bits per second. The
// bucket is sized so a single io.Copy read chunk (32 KiB) or a maximum-size
// UDP datagram (64 KiB) never exceeds it, which keeps the read-then-wait
// pattern below burst-safe.
func newByteLimiter(bps int64) *rate.Limiter {
	if bps <= 0 {
		return nil
	}
	bytesPerSec := bps / 8
	if bytesPerSec < 1 {
		bytesPerSec = 1
	}
	burst := int(bytesPerSec)
	if burst < 64*1024 {
		burst = 64 * 1024
	}
	return rate.NewLimiter(rate.Limit(bytesPerSec), burst)
}

// limitReader paces reads from r to at most bps bits per second. A nil or
// non-positive bps returns r unchanged.
func limitReader(r io.Reader, bps int64) io.Reader {
	l := newByteLimiter(bps)
	if l == nil {
		return r
	}
	return &limitedReader{r: r, l: l, burst: l.Burst()}
}

type limitedReader struct {
	r     io.Reader
	l     *rate.Limiter
	burst int
}

func (lr *limitedReader) Read(p []byte) (int, error) {
	// Never read more than the bucket can hold, otherwise WaitN fails.
	if len(p) > lr.burst {
		p = p[:lr.burst]
	}
	n, err := lr.r.Read(p)
	if n > 0 {
		if werr := lr.l.WaitN(context.Background(), n); werr != nil {
			return 0, werr
		}
	}
	return n, err
}
