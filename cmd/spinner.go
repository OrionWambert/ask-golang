package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// spinner shows activity on a terminal. It stays silent when w is piped, and a
// nil spinner is valid.
type spinner struct {
	w    *os.File
	quit chan struct{}
	done chan struct{}
	once sync.Once
}

func startSpinner(w *os.File) *spinner {
	if !isTerminal(w) {
		return nil
	}
	s := &spinner{w: w, quit: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s
}

func (s *spinner) run() {
	defer close(s.done)
	frames := `|/-\`
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; ; i++ {
		fmt.Fprintf(s.w, "\r%c thinking", frames[i%len(frames)])
		select {
		case <-ticker.C:
		case <-s.quit:
			fmt.Fprint(s.w, "\r          \r")
			return
		}
	}
}

func (s *spinner) stop() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.quit) })
	<-s.done
}
