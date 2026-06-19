package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"golang.org/x/sys/unix"
)

type (
	// TerminalPollReader supplies a file descriptor and handles readiness events from TerminalPoll.
	TerminalPollReader interface {
		FD() int
		Handle(fd int, buf []byte, isHup, closing bool) (done bool)
	}

	// TerminalPoll multiplexes terminal-related file descriptors until context cancellation or handler completion.
	TerminalPoll interface {
		Close() error
		Add(fds ...int) error
		Remove(fds ...int) error
		Register(cb ...TerminalPollReader) error
		Wait(ctx context.Context)
	}
	// terminalPoll stores platform poller state and registered handlers.
	terminalPoll struct {
		kqfd        int
		cancelIdent uint64

		events  [32]unix.Kevent_t
		handler map[int][]TerminalPollReader

		l sync.Mutex
	}
	// simpleCopier copies bytes from a watched descriptor to an io.Writer.
	simpleCopier struct {
		fd      int
		w       io.Writer
		pending []byte
	}
	// escapeHandler recognizes the console escape sequence used to terminate PTY forwarding.
	escapeHandler struct {
		fd   int
		step int
	}
)

// NewEscapeHandler returns a poll reader that completes when the escape sequence is observed.
func NewEscapeHandler(fd int) TerminalPollReader {
	return &escapeHandler{
		fd: fd,
	}
}

// FD returns the file descriptor watched by the terminal poller.
func (h *escapeHandler) FD() int { return h.fd }

// Handle processes an event delivered by the terminal poller.
func (h *escapeHandler) Handle(_ int, buf []byte, isHup, _ bool) bool {
	for _, b := range buf {
		switch h.step {
		case 0:
			switch b {
			case 0x1b:
				h.step++
			}
		case 1:
			switch b {
			case 0x28:
				return true
			}
			h.step = 0
		default:
		}
	}
	return isHup
}

// NewTerminalPollCopier returns a poll reader that forwards descriptor bytes to the writer.
func NewTerminalPollCopier(fd int, w io.Writer) TerminalPollReader {
	return &simpleCopier{
		fd: fd,
		w:  w,
	}
}

// FD returns the file descriptor watched by the terminal poller.
func (c *simpleCopier) FD() int { return c.fd }

func (c *simpleCopier) HasPending() bool { return len(c.pending) > 0 }

// Handle processes an event delivered by the terminal poller.
func (c *simpleCopier) Handle(_ int, buf []byte, isHup, closing bool) bool {
	if len(c.pending) > 0 {
		combined := make([]byte, 0, len(c.pending)+len(buf))
		combined = append(combined, c.pending...)
		combined = append(combined, buf...)
		buf = combined
		c.pending = nil
	}

	for offset := 0; offset < len(buf); {

		w, err := c.w.Write(buf[offset:])
		offset += w

		if errors.Is(err, io.EOF) {
			InfoLog.Printf("closing")
			return true
		} else if errors.Is(err, unix.EAGAIN) {
			// EAGAIN means the descriptor should be retried by a later poll event.
			c.pending = append(c.pending[:0], buf[offset:]...)
			return closing
		} else if err != nil {
			ErrLog.Printf("failed to copy data: %s", err)
			return true
		}
		if w == 0 {
			c.pending = append(c.pending[:0], buf[offset:]...)
			return closing
		}
	}

	return isHup || closing && len(buf) == 0
}

// NewTerminalPoll creates the platform terminal poller implementation.
func NewTerminalPoll() (TerminalPoll, error) {
	kqfd, e := unix.Kqueue()
	if e != nil {
		return nil, fmt.Errorf("epoll_create1: %w", e)
	}

	return &terminalPoll{
		kqfd:    kqfd,
		handler: make(map[int][]TerminalPollReader),
	}, nil
}

// Close releases resources held by the receiver.
func (p *terminalPoll) Close() (err error) {
	p.l.Lock()
	defer p.l.Unlock()
	kqfd := p.kqfd
	if kqfd == 0 {
		return nil
	}
	p.kqfd = 0
	defer func() {
		err = errors.Join(err, unix.Close(kqfd))
	}()

	for fd := range p.handler {
		delete(p.handler, fd)

		ev := unix.Kevent_t{
			Ident:  uint64(fd),
			Filter: unix.EVFILT_READ,
			Flags:  unix.EV_DELETE,
		}
		if _, deleteErr := unix.Kevent(kqfd, []unix.Kevent_t{ev}, nil, nil); err != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove EPoll with fd(%d) epfs(%d) : %v", fd, kqfd, deleteErr))
			continue
		}
	}

	return
}

// Add registers file descriptors with the platform poller.
func (p *terminalPoll) Add(fds ...int) error {
	p.l.Lock()
	defer p.l.Unlock()

	kqfd := p.kqfd
	if kqfd == 0 {
		return fmt.Errorf("kqfd not opened")
	}

	var (
		loopErrs   error
		added      []int
		removeErrs error
	)

	for _, fd := range fds {
		err := unix.SetNonblock(fd, true)
		if err != nil {
			loopErrs = errors.Join(loopErrs, fmt.Errorf("set nonblock failed on fd(%d): %w", fd, err))
			break
		}

		ev := unix.Kevent_t{
			Ident:  uint64(fd),
			Filter: unix.EVFILT_READ,
			Flags:  unix.EV_ADD,
		}
		if _, err = unix.Kevent(kqfd, []unix.Kevent_t{ev}, nil, nil); err != nil {
			loopErrs = errors.Join(loopErrs, fmt.Errorf("failed to adding Kevent with fd(%d) epfs(%d) : %v", fd, kqfd, err))
			break
		}
		added = append(added, fd)
	}

	if loopErrs != nil {
		fd := -1
		for len(added) > 0 {
			fd, added = added[len(added)-1], added[:len(added)-1]

			ev := unix.Kevent_t{
				Ident:  uint64(fd),
				Filter: unix.EVFILT_READ,
				Flags:  unix.EV_DELETE,
			}
			if _, err := unix.Kevent(kqfd, []unix.Kevent_t{ev}, nil, nil); err != nil {
				removeErrs = errors.Join(removeErrs, fmt.Errorf("failed to remove Kevent with fd(%d) epfs(%d) : %v", fd, kqfd, err))
			}
		}
	} else {
		for _, fd := range added {
			if _, added := p.handler[fd]; !added {
				p.handler[fd] = nil
			}
		}
	}

	return errors.Join(loopErrs, removeErrs)
}

// Remove unregisters file descriptors from the platform poller.
func (p *terminalPoll) Remove(fds ...int) error {

	p.l.Lock()
	defer p.l.Unlock()

	return p.removeInternal(fds...)
}

// Register attaches terminal poll handlers to their file descriptors.
func (p *terminalPoll) Register(pr ...TerminalPollReader) error {
	p.l.Lock()
	defer p.l.Unlock()

	if p.handler == nil {
		return fmt.Errorf("terminalPoll not opened")
	}

	for _, h := range pr {
		fd := h.FD()
		callbacks, _ := p.handler[fd]
		p.handler[fd] = append(callbacks, h)
	}

	return nil
}

// Wait runs the terminal poll loop until the context is done or handlers complete.
func (p *terminalPoll) Wait(ctx context.Context) {
	kqfd := p.kqfd
	if kqfd == 0 {
		return
	}

	p.cancelIdent = ^uint64(0)
	if _, err := unix.Kevent(kqfd, []unix.Kevent_t{{Ident: p.cancelIdent, Filter: unix.EVFILT_USER, Flags: unix.EV_ADD | unix.EV_CLEAR}}, nil, nil); err != nil {
		p.cancelIdent = 0
	} else {
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_, _ = unix.Kevent(kqfd, []unix.Kevent_t{{Ident: p.cancelIdent, Filter: unix.EVFILT_USER, Fflags: unix.NOTE_TRIGGER}}, nil, nil)
			case <-done:
			}
		}()
		defer close(done)
	}

	buf := [512]byte{}

	closing := false
	for !closing {
		closing = p.wait(ctx, kqfd, buf, closing)
	}
}

// wait performs one platform poll operation and dispatches ready events.
func (p *terminalPoll) wait(ctx context.Context, kqfd int, buf [512]byte, closing bool) bool {
	p.l.Lock()
	defer p.l.Unlock()

	select {
	case <-ctx.Done():
		return true
	default:
	}

	// Poll registered file descriptors for terminal readiness events.
	var timeout *unix.Timespec
	if p.cancelIdent == 0 || p.hasPendingLocked() {
		timeout = &unix.Timespec{Nsec: 100 * 1000 * 1000}
	}
	n, errno := unix.Kevent(kqfd, nil, p.events[:], timeout)
	switch errno {
	case nil:
		if n >= len(p.events) {
			ErrLog.Printf("epoll_wait: returned more than %d events", n)
			return true
		} else if n > 0 {
			break
		}
		// No readiness event means the loop should retry pending writes and continue waiting.
		closing = p.flushPendingLocked(closing)
		select {
		case <-ctx.Done():
			return true
		default:
			return closing
		}
	case unix.EINTR:
		runtime.Gosched()
		select {
		case <-ctx.Done():
			return true
		default:
			return closing
		}
	default:
		ErrLog.Printf("epoll_wait: error :%s ", errno)
		return true
	}

	// Dispatch process exit readiness or context cancellation events.
	for _, e := range p.events[:n] {
		if e.Filter == unix.EVFILT_USER && e.Ident == p.cancelIdent {
			return true
		}
		fd := int(e.Ident)
		isHup := (e.Flags & unix.EV_EOF) != 0

		var (
			n   int
			err error
		)
		if (e.Filter & unix.EVFILT_READ) != 0 {
			n, err = unix.Read(fd, buf[:])
			if errors.Is(err, io.EOF) {
			} else if err != nil {
				ErrLog.Printf("epoll_wait: error :%s ", err)
			}
		}

		callbacks, _ := p.handler[fd]
		for _, cb := range callbacks {
			closing = cb.Handle(fd, buf[:n], isHup, closing) || closing
		}

		if isHup && !p.hasPendingCallbacksLocked(callbacks) {
			err = p.removeInternal(fd)
			if err != nil {
				ErrLog.Printf("%s", err)
			}
		}
	}

	// Check for done signal
	select {
	case <-ctx.Done():
		return true
	default:
		return closing
	}
}

func (p *terminalPoll) hasPendingCallbacksLocked(callbacks []TerminalPollReader) bool {
	for _, cb := range callbacks {
		pending, ok := cb.(interface{ HasPending() bool })
		if ok && pending.HasPending() {
			return true
		}
	}
	return false
}

func (p *terminalPoll) hasPendingLocked() bool {
	for _, callbacks := range p.handler {
		for _, cb := range callbacks {
			pending, ok := cb.(interface{ HasPending() bool })
			if ok && pending.HasPending() {
				return true
			}
		}
	}
	return false
}

func (p *terminalPoll) flushPendingLocked(closing bool) bool {
	for fd, callbacks := range p.handler {
		for _, cb := range callbacks {
			pending, ok := cb.(interface{ HasPending() bool })
			if !ok || !pending.HasPending() {
				continue
			}
			closing = cb.Handle(fd, nil, false, closing) || closing
		}
	}
	return closing
}

// removeInternal removes a descriptor from the poller without taking the public lock.
func (p *terminalPoll) removeInternal(fds ...int) error {
	kqfd := p.kqfd
	if kqfd == 0 {
		return fmt.Errorf("kqfd not opened")
	}

	var (
		removed    []int
		removeErrs error
	)
	fd := -1
	for len(fds) > 0 {
		fd, fds = fds[len(fds)-1], fds[:len(fds)-1]

		ev := unix.Kevent_t{
			Ident:  uint64(fd),
			Filter: unix.EVFILT_READ,
			Flags:  unix.EV_DELETE,
		}
		if _, err := unix.Kevent(kqfd, []unix.Kevent_t{ev}, nil, nil); err != nil {
			removeErrs = errors.Join(removeErrs, fmt.Errorf("failed to remove Kevent with fd(%d) epfs(%d) : %v", fd, kqfd, err))
			continue
		}

		removed = append(removed, fd)
	}

	for _, fd = range removed {
		if _, added := p.handler[fd]; added {
			delete(p.handler, fd)
		}
	}

	return removeErrs
}
