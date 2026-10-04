package runner

import (
	"bytes"
	"context"
	"errors"
)

const maxOutputBytes = 1 << 20

var ErrOutputLimit = errors.New("command output exceeds 1 MiB per stream")

// Each stream has its own buffer and writer goroutine.
type outputBuffer struct {
	buffer   bytes.Buffer
	cancel   context.CancelFunc
	exceeded bool
}

func (b *outputBuffer) Write(p []byte) (int, error) {
	if len(p) > maxOutputBytes-b.buffer.Len() {
		b.exceeded = true
		b.cancel()
		return 0, ErrOutputLimit
	}
	return b.buffer.Write(p)
}
