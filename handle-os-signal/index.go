package handleossignal

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"code.finan.one/finan-one-be/fo-utils/l"
)

// IShutdownHandler ...
type IShutdownHandler interface {
	SetTimeout(t time.Duration)
	SetLogger(ll l.Logger)
	HandleDefer(f func())
	Handle()
}

type handler struct {
	closes             []func()
	mu                 sync.Mutex
	delayForceShutdown time.Duration
	ll                 l.Logger
}

// New ... new handle os signal
func New(ll l.Logger) *handler {
	return &handler{
		delayForceShutdown: 15,
		ll:                 ll,
	}
}

// SetTimeout ...set delay time force shutdown in second. default is 15s
func (h *handler) SetTimeout(t time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.delayForceShutdown = t
}

// SetLogger ...
func (h *handler) SetLogger(ll l.Logger) {
	h.ll = ll
}

// HandleDefer ...register clean-up actions when shutdown
func (h *handler) HandleDefer(f func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closes = append(h.closes, f)
}

// Handle ...waiting for a signal and do clean actions
func (h *handler) Handle() {
	// handle signal
	osSignal := make(chan os.Signal, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)
	<-osSignal

	h.ll.Info("Shutdown started...")
	count := len(h.closes)
	if count == 0 {
		h.ll.Info("Bye ^^")
		return
	}
	// doneCloses
	doneCloses := make(chan struct{})
	for i := len(h.closes) - 1; i >= 0; i-- {
		f := h.closes[i]
		go h.closeObj(f, doneCloses)
	}
	timer := time.NewTimer(h.delayForceShutdown * time.Second)
	for {
		select {
		case <-timer.C:
			h.ll.Fatal("Force shutdown due to timeout!")
		case <-doneCloses:
			count--
			if count > 0 {
				continue
			}
			h.ll.Info("Bye ^^")
			os.Exit(0)
			return
		}
	}
}

func (h *handler) closeObj(closer func(), done chan struct{}) {
	closer()
	done <- struct{}{}
}
