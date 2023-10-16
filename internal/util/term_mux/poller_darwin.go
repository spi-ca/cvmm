package term_mux

import (
	"amuz.es/src/spi-ca/chmgr/internal/util"
	"context"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"runtime"
	"sync"
)

type (
	TerminalPollReader interface {
		FD() int
		Handle(fd int, buf []byte, isHup, closing bool) (done bool)
	}

	TerminalPoll interface {
		Close() error
		Add(fds ...int) error
		Remove(fds ...int) error
		Register(cb ...TerminalPollReader) error
		Wait(ctx context.Context)
	}
	terminalPoll struct {
		kqfd int

		events  [32]unix.Kevent_t
		handler map[int][]TerminalPollReader

		l sync.Mutex
	}
	simpleCopier struct {
		fd int
		w  io.Writer
	}
	escapeHandler struct {
		fd   int
		step int
	}
)

func NewEscapeHandler(fd int) TerminalPollReader {
	return &escapeHandler{
		fd: fd,
	}
}

func (h *escapeHandler) FD() int { return h.fd }
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

func NewTerminalPollCopier(fd int, w io.Writer) TerminalPollReader {
	return &simpleCopier{
		fd: fd,
		w:  w,
	}
}
func (c simpleCopier) FD() int { return c.fd }
func (c simpleCopier) Handle(_ int, buf []byte, isHup, closing bool) bool {
	for offset := 0; offset < len(buf); {
		w, err := c.w.Write(buf[offset:])
		offset += w

		if errors.Is(err, io.EOF) {
			util.InfoLog.Printf("closing")
			return true
		} else if err != nil {
			util.ErrLog.Printf("failed to copy data: %s", err)
			return true
		}
	}

	return isHup || closing && len(buf) == 0
}

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

func (p *terminalPoll) Remove(fds ...int) error {

	p.l.Lock()
	defer p.l.Unlock()

	return p.removeInternal(fds...)
}

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

func (p *terminalPoll) Wait(ctx context.Context) {
	kqfd := p.kqfd
	if kqfd == 0 {
		//return fmt.Errorf("kqfd not opened")
		return
	}

	buf := [512]byte{}

	closing := false
	for !closing {
		closing = p.wait(ctx, kqfd, buf, closing)
	}
}

func (p *terminalPoll) wait(ctx context.Context, kqfd int, buf [512]byte, closing bool) bool {
	p.l.Lock()
	defer p.l.Unlock()

	// Poll the file descriptor
	n, errno := unix.Kevent(kqfd, nil, p.events[:], nil)
	switch errno {
	case nil:
		if n >= len(p.events) {
			util.ErrLog.Printf("epoll_wait: returned more than %d events", n)
			return true
		} else if n > 0 {
			break
		}
		// if n <=0
		fallthrough
	case unix.EINTR:
		runtime.Gosched()
		return closing
	default:
		util.ErrLog.Printf("epoll_wait: error :%s ", errno)
		return true
	}

	// Process events
	for _, e := range p.events[:n] {
		fd := int(e.Ident)
		isHup := (e.Flags & unix.EV_EOF) != 0

		var (
			n   int
			err error
		)
		if (e.Filter & unix.EVFILT_READ) > 0 {
			n, err = unix.Read(fd, buf[:])
			if errors.Is(err, io.EOF) {
			} else if err != nil {
				util.ErrLog.Printf("epoll_wait: error :%s ", err)
			}
		}
		callbacks, _ := p.handler[fd]
		for _, cb := range callbacks {
			closing = cb.Handle(fd, buf[:n], isHup, closing) || closing
		}

		if isHup {
			err = p.removeInternal(fd)
			if err != nil {
				util.ErrLog.Printf("%s", err)
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
