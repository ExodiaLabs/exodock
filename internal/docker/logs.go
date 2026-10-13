package docker

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
	sdk "github.com/moby/moby/client"
)

const maxLogLine = 16 << 10
const maxLogFrame = 16 << 20

func (c *Client) Logs(ctx context.Context, id string, options core.LogOptions) (core.LogStream, error) {
	if options.Stream != "" && options.Stream != "all" && options.Stream != "stdout" && options.Stream != "stderr" {
		return nil, errors.New("Choose all, stdout or stderr logs")
	}
	inspectCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	result, err := c.engine.ContainerInspect(inspectCtx, id, sdk.ContainerInspectOptions{})
	cancel()
	if err != nil {
		return nil, safeError(err)
	}
	tty := result.Container.Config != nil && result.Container.Config.Tty
	if tty && options.Stream == "stderr" {
		return nil, errors.New("TTY containers combine stdout and stderr; choose all or stdout")
	}
	tail := options.Tail
	if tail <= 0 {
		tail = 200
	}
	if tail > 10000 {
		tail = 10000
	}
	opts := sdk.ContainerLogsOptions{Follow: true, Tail: strconv.Itoa(tail), ShowStdout: options.Stream != "stderr", ShowStderr: options.Stream != "stdout"}
	if !options.Since.IsZero() {
		opts.Since = options.Since.UTC().Format(time.RFC3339Nano)
	}
	streamCtx, cancelStream := context.WithCancel(ctx)
	timer := time.AfterFunc(15*time.Second, cancelStream)
	body, err := c.engine.ContainerLogs(streamCtx, id, opts)
	timer.Stop()
	if err != nil {
		timedOut := ctx.Err() == nil && streamCtx.Err() != nil
		cancelStream()
		if timedOut {
			return nil, errors.New("Docker logs request timed out; reconnect to resume")
		}
		return nil, safeError(err)
	}
	return newLogReader(&cancelLogBody{ReadCloser: body, cancel: cancelStream}, tty), nil
}

// logReader decodes incrementally with two bounded per-stream lines and no producer goroutine.
// Next has one consumer; Close is safe concurrently with Next and cancels a blocked read.
type logReader struct {
	body      io.ReadCloser
	reader    *bufio.Reader
	tty       bool
	once      sync.Once
	closeErr  error
	remaining uint32
	stream    byte
	lines     [3][]byte
	truncated [3]bool
	ended     bool
}

func newLogReader(body io.ReadCloser, tty bool) *logReader {
	return &logReader{body: body, reader: bufio.NewReaderSize(body, 4096), tty: tty}
}
func (l *logReader) Close() error {
	l.once.Do(func() { l.closeErr = l.body.Close() })
	return l.closeErr
}
func (l *logReader) entry(stream byte) core.LogEntry {
	text := string(l.lines[stream])
	l.lines[stream] = l.lines[stream][:0]
	if l.truncated[stream] {
		text += " [line truncated]"
		l.truncated[stream] = false
	}
	name := "stdout"
	if stream == 2 {
		name = "stderr"
	}
	return core.LogEntry{Text: safety.Clean(text), Stream: name}
}
func (l *logReader) Next(ctx context.Context) (core.LogEntry, error) {
	if err := ctx.Err(); err != nil {
		return core.LogEntry{}, err
	}
	stop := context.AfterFunc(ctx, func() { _ = l.Close() })
	defer stop()
	for {
		if l.ended {
			for _, stream := range []byte{1, 2} {
				if len(l.lines[stream]) > 0 || l.truncated[stream] {
					return l.entry(stream), nil
				}
			}
			return core.LogEntry{}, io.EOF
		}
		stream := byte(1)
		if !l.tty {
			if l.remaining == 0 {
				var header [8]byte
				_, err := io.ReadFull(l.reader, header[:])
				if err != nil {
					if ctx.Err() != nil {
						return core.LogEntry{}, ctx.Err()
					}
					if err == io.EOF {
						l.ended = true
						continue
					}
					return core.LogEntry{}, errors.New("Docker log stream ended mid-frame; reconnect to resume")
				}
				l.stream = header[0]
				l.remaining = binary.BigEndian.Uint32(header[4:])
				if (l.stream != 1 && l.stream != 2) || header[1] != 0 || header[2] != 0 || header[3] != 0 || l.remaining > maxLogFrame {
					_ = l.Close()
					return core.LogEntry{}, errors.New("Docker returned an unsupported or oversized log frame")
				}
				if l.remaining == 0 {
					continue
				}
			}
			stream = l.stream
		}
		b, err := l.reader.ReadByte()
		if err != nil {
			if ctx.Err() != nil {
				return core.LogEntry{}, ctx.Err()
			}
			if err == io.EOF && l.tty {
				l.ended = true
				continue
			}
			return core.LogEntry{}, errors.New("Docker log stream was interrupted; reconnect to resume")
		}
		if !l.tty {
			l.remaining--
		}
		if b == '\n' {
			return l.entry(stream), nil
		}
		if len(l.lines[stream]) < maxLogLine {
			l.lines[stream] = append(l.lines[stream], b)
		} else {
			l.truncated[stream] = true
		}
	}
}

// Closing a subscription also releases the context retained by the SDK reader.
type cancelLogBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelLogBody) Close() error { b.cancel(); return b.ReadCloser.Close() }
