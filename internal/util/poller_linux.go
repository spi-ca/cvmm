package util

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"runtime"
	"sync"
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
		waitMsec int

		epfd int

		events  [32]unix.EpollEvent
		handler map[int][]TerminalPollReader

		l sync.Mutex
	}
	// simpleCopier copies bytes from a watched descriptor to an io.Writer.
	simpleCopier struct {
		fd int
		w  io.Writer
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
func (c simpleCopier) FD() int { return c.fd }

// Handle processes an event delivered by the terminal poller.
func (c simpleCopier) Handle(_ int, buf []byte, isHup, closing bool) bool {
	for offset := 0; offset < len(buf); {
		w, err := c.w.Write(buf[offset:])
		offset += w

		if errors.Is(err, io.EOF) {
			InfoLog.Printf("closing")
			return true
		} else if err != nil {
			ErrLog.Printf("failed to copy data: %s", err)
			return true
		}
	}

	return isHup || closing && len(buf) == 0
}

// NewTerminalPoll creates the platform terminal poller implementation.
func NewTerminalPoll() (TerminalPoll, error) {
	epfd, e := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if e != nil {
		return nil, fmt.Errorf("epoll_create1: %w", e)
	}

	return &terminalPoll{
		epfd:    epfd,
		handler: make(map[int][]TerminalPollReader),
	}, nil
}

// Close releases resources held by the receiver.
func (p *terminalPoll) Close() (err error) {
	p.l.Lock()
	defer p.l.Unlock()
	epfd := p.epfd
	if epfd == 0 {
		return nil
	}
	p.epfd = 0
	defer func() {
		err = errors.Join(err, unix.Close(epfd))
	}()

	for fd := range p.handler {
		delete(p.handler, fd)

		ev := unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLHUP, Fd: int32(fd)}
		if deleteErr := unix.EpollCtl(epfd, unix.EPOLL_CTL_DEL, fd, &ev); err != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove EPoll with fd(%d) epfs(%d) : %v", fd, epfd, deleteErr))
			continue
		}
	}

	return
}

// Add registers file descriptors with the platform poller.
func (p *terminalPoll) Add(fds ...int) error {
	p.l.Lock()
	defer p.l.Unlock()

	epfd := p.epfd
	if epfd == 0 {
		return fmt.Errorf("epfd not opened")
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

		ev := unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLHUP, Fd: int32(fd)}
		if err = unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, &ev); err != nil {
			loopErrs = errors.Join(loopErrs, fmt.Errorf("failed to adding EPoll with fd(%d) epfs(%d) : %v", fd, epfd, err))
			break
		}
		added = append(added, fd)
	}

	if loopErrs != nil {
		fd := -1
		for len(added) > 0 {
			fd, added = added[len(added)-1], added[:len(added)-1]

			ev := unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLHUP, Fd: int32(fd)}
			if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_DEL, fd, &ev); err != nil {
				removeErrs = errors.Join(removeErrs, fmt.Errorf("failed to remove EPoll with fd(%d) epfs(%d) : %v", fd, epfd, err))
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
	epfd := p.epfd
	if epfd == 0 {
		return
	}

	p.waitMsec = -1

	buf := [512]byte{}

	closing := false
	for !closing {
		closing = p.wait(ctx, epfd, buf, closing)
	}
}

// wait performs one platform poll operation and dispatches ready events.
func (p *terminalPoll) wait(ctx context.Context, epfd int, buf [512]byte, closing bool) bool {
	p.l.Lock()
	defer p.l.Unlock()

	// Poll registered file descriptors for terminal readiness events.
	n, errno := unix.EpollWait(epfd, p.events[:], p.waitMsec)
	switch errno {
	case nil:
		if n >= len(p.events) {
			ErrLog.Printf("epoll_wait: returned more than %d events", n)
			return true
		} else if n > 0 {
			p.waitMsec = 0
			break
		}
		// No readiness event means the poll loop should continue waiting.
		fallthrough
	case unix.EINTR:
		runtime.Gosched()
		p.waitMsec = -1
		return closing
	default:
		ErrLog.Printf("epoll_wait: error :%s ", errno)
		return true
	}

	// Dispatch each ready event to the handlers registered for its file descriptor.
	for _, e := range p.events[:n] {
		fd := int(e.Fd)
		isHup := (e.Events & unix.EPOLLHUP) != 0

		var (
			n   int
			err error
		)
		if (e.Events & unix.EPOLLIN) != 0 {
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

		if isHup {
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

// removeInternal removes a descriptor from the poller without taking the public lock.
func (p *terminalPoll) removeInternal(fds ...int) error {
	epfd := p.epfd
	if epfd == 0 {
		return fmt.Errorf("epfd not opened")
	}

	var (
		removed    []int
		removeErrs error
	)
	fd := -1
	for len(fds) > 0 {
		fd, fds = fds[len(fds)-1], fds[:len(fds)-1]

		ev := unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLHUP, Fd: int32(fd)}
		if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_DEL, fd, &ev); err != nil {
			removeErrs = errors.Join(removeErrs, fmt.Errorf("failed to remove EPoll with fd(%d) epfs(%d) : %v", fd, epfd, err))
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
