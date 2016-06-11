// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements runtime support for signal handling.
//
// Most synchronization primitives are not available from
// the signal handler (it cannot block, allocate memory, or use locks)
// so the handler communicates with a processing goroutine
// via struct sig, below.
//
// sigsend is called by the signal handler to queue a new signal.
// signal_recv is called by the Go program to receive a newly queued signal.
// Synchronization between sigsend and signal_recv is based on the sig.state
// variable. It can be in 3 states: sigIdle, sigReceiving and sigSending.
// sigReceiving means that signal_recv is blocked on sig.Note and there are no
// new pending signals.
// sigSending means that sig.mask *may* contain new pending signals,
// signal_recv can't be blocked in this state.
// sigIdle means that there are no new pending signals and signal_recv is not blocked.
// Transitions between states are done atomically with CAS.
// When signal_recv is unblocked, it resets sig.Note and rechecks sig.mask.
// If several sigsends and signal_recv execute concurrently, it can lead to
// unnecessary rechecks of sig.mask, but it cannot lead to missed signals
// nor deadlocks.

// +build !plan9

package runtime

import (
	"runtime/internal/atomic"
	_ "unsafe" // for go:linkname
)

var sig struct {
	note    note
	mask    [(_NSIG + 31) / 32]uint32
	wanted  [(_NSIG + 31) / 32]uint32
	ignored [(_NSIG + 31) / 32]uint32
	recv    [(_NSIG + 31) / 32]uint32
	state   uint32
	inuse   bool
}

const (
	sigIdle = iota
	sigReceiving
	sigSending
)

// Called from sighandler to send a signal back out of the signal handling thread.
// Reports whether the signal was sent. If not, the caller typically crashes the program.
func sigsend(s uint32) bool {
	bit := uint32(1) << uint(s&31)
	if !sig.inuse || s >= uint32(32*len(sig.wanted)) || sig.wanted[s/32]&bit == 0 {
		return false
	}

	// Add signal to outgoing queue.
	for {
		mask := sig.mask[s/32]
		if mask&bit != 0 {
			return true // signal already in queue
		}
		if atomic.Cas(&sig.mask[s/32], mask, mask|bit) {
			break
		}
	}

	// Notify receiver that queue has new bit.
Send:
	for {
		switch atomic.Load(&sig.state) {
		default:
			throw("sigsend: inconsistent state")
		case sigIdle:
			if atomic.Cas(&sig.state, sigIdle, sigSending) {
				break Send
			}
		case sigSending:
			// notification already pending
			break Send
		case sigReceiving:
			if atomic.Cas(&sig.state, sigReceiving, sigIdle) {
				notewakeup(&sig.note)
				break Send
			}
		}
	}

	return true
}

// Called to receive the next queued signal.
// Must only be called from a single goroutine at a time.
//go:linkname signal_recv os/signal.signal_recv
func signal_recv() uint32 {
	for {
		// Serve any signals from local copy.
		for i := uint32(0); i < _NSIG; i++ {
			if sig.recv[i/32]&(1<<(i&31)) != 0 {
				sig.recv[i/32] &^= 1 << (i & 31)
				return i
			}
		}

		// Wait for updates to be available from signal sender.
	Receive:
		for {
			switch atomic.Load(&sig.state) {
			default:
				throw("signal_recv: inconsistent state")
			case sigIdle:
				if atomic.Cas(&sig.state, sigIdle, sigReceiving) {
					notetsleepg(&sig.note, -1)
					noteclear(&sig.note)
					break Receive
				}
			case sigSending:
				if atomic.Cas(&sig.state, sigSending, sigIdle) {
					break Receive
				}
			}
		}

		// Incorporate updates from sender into local copy.
		for i := range sig.mask {
			sig.recv[i] = atomic.Xchg(&sig.mask[i], 0)
		}
	}
}

// Must only be called from a single goroutine at a time.
//go:linkname signal_enable os/signal.signal_enable
func signal_enable(s uint32) {
	if !sig.inuse {
		// The first call to signal_enable is for us
		// to use for initialization. It does not pass
		// signal information in m.
		sig.inuse = true // enable reception of signals; cannot disable
		noteclear(&sig.note)
		return
	}

	if s >= uint32(len(sig.wanted)*32) {
		return
	}
	sig.wanted[s/32] |= 1 << (s & 31)
	sig.ignored[s/32] &^= 1 << (s & 31)
	sigenable(s)
}

// Must only be called from a single goroutine at a time.
//go:linkname signal_disable os/signal.signal_disable
func signal_disable(s uint32) {
	if s >= uint32(len(sig.wanted)*32) {
		return
	}
	sig.wanted[s/32] &^= 1 << (s & 31)
	sigdisable(s)
}

// Must only be called from a single goroutine at a time.
//go:linkname signal_ignore os/signal.signal_ignore
func signal_ignore(s uint32) {
	if s >= uint32(len(sig.wanted)*32) {
		return
	}
	sig.wanted[s/32] &^= 1 << (s & 31)
	sig.ignored[s/32] |= 1 << (s & 31)
	sigignore(s)
}

// Checked by signal handlers.
func signal_ignored(s uint32) bool {
	return sig.ignored[s/32]&(1<<(s&31)) != 0
}
