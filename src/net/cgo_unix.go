// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build cgo,!netgo
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package net

/*
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netdb.h>
#include <stdlib.h>
#include <unistd.h>
#include <string.h>
*/
import "C"

import (
	"syscall"
	"unsafe"
)

// An addrinfoErrno represents a getaddrinfo, getnameinfo-specific
// error number. It's a signed number and a zero value is a non-error
// by convention.
type addrinfoErrno int

func (eai addrinfoErrno) Error() string   { return C.GoString(C.gai_strerror(C.int(eai))) }
func (eai addrinfoErrno) Temporary() bool { return eai == C.EAI_AGAIN }
func (eai addrinfoErrno) Timeout() bool   { return false }

func cgoLookupHost(name string) (hosts []string, err error, completed bool) {
	addrs, err, completed := cgoLookupIP(name)
	for _, addr := range addrs {
		hosts = append(hosts, addr.String())
	}
	return
}

func cgoLookupPort(network, service string) (port int, err error, completed bool) {
	acquireThread()
	defer releaseThread()

	var hints C.struct_addrinfo
	switch network {
	case "": // no hints
	case "tcp", "tcp4", "tcp6":
		hints.ai_socktype = C.SOCK_STREAM
		hints.ai_protocol = C.IPPROTO_TCP
	case "udp", "udp4", "udp6":
		hints.ai_socktype = C.SOCK_DGRAM
		hints.ai_protocol = C.IPPROTO_UDP
	default:
		return 0, &DNSError{Err: "unknown network", Name: network + "/" + service}, true
	}
	if len(network) >= 4 {
		switch network[3] {
		case '4':
			hints.ai_family = C.AF_INET
		case '6':
			hints.ai_family = C.AF_INET6
		}
	}

	s := C.CString(service)
	var res *C.struct_addrinfo
	defer C.free(unsafe.Pointer(s))
	gerrno, err := C.getaddrinfo(nil, s, &hints, &res)
	if gerrno != 0 {
		switch gerrno {
		case C.EAI_SYSTEM:
			if err == nil { // see golang.org/issue/6232
				err = syscall.EMFILE
			}
		default:
			err = addrinfoErrno(gerrno)
		}
		return 0, &DNSError{Err: err.Error(), Name: network + "/" + service}, true
	}
	defer C.freeaddrinfo(res)

	for r := res; r != nil; r = r.ai_next {
		switch r.ai_family {
		case C.AF_INET:
			sa := (*syscall.RawSockaddrInet4)(unsafe.Pointer(r.ai_addr))
			p := (*[2]byte)(unsafe.Pointer(&sa.Port))
			return int(p[0])<<8 | int(p[1]), nil, true
		case C.AF_INET6:
			sa := (*syscall.RawSockaddrInet6)(unsafe.Pointer(r.ai_addr))
			p := (*[2]byte)(unsafe.Pointer(&sa.Port))
			return int(p[0])<<8 | int(p[1]), nil, true
		}
	}
	return 0, &DNSError{Err: "unknown port", Name: network + "/" + service}, true
}

func cgoLookupIPCNAME(name string) (addrs []IPAddr, cname string, err error, completed bool) {
	acquireThread()
	defer releaseThread()

	var hints C.struct_addrinfo
	hints.ai_flags = cgoAddrInfoFlags
	hints.ai_socktype = C.SOCK_STREAM

	h := C.CString(name)
	defer C.free(unsafe.Pointer(h))
	var res *C.struct_addrinfo
	gerrno, err := C.getaddrinfo(h, nil, &hints, &res)
	if gerrno != 0 {
		switch gerrno {
		case C.EAI_SYSTEM:
			if err == nil {
				// err should not be nil, but sometimes getaddrinfo returns
				// gerrno == C.EAI_SYSTEM with err == nil on Linux.
				// The report claims that it happens when we have too many
				// open files, so use syscall.EMFILE (too many open files in system).
				// Most system calls would return ENFILE (too many open files),
				// so at the least EMFILE should be easy to recognize if this
				// comes up again. golang.org/issue/6232.
				err = syscall.EMFILE
			}
		case C.EAI_NONAME:
			err = errNoSuchHost
		default:
			err = addrinfoErrno(gerrno)
		}
		return nil, "", &DNSError{Err: err.Error(), Name: name}, true
	}
	defer C.freeaddrinfo(res)

	if res != nil {
		cname = C.GoString(res.ai_canonname)
		if cname == "" {
			cname = name
		}
		if len(cname) > 0 && cname[len(cname)-1] != '.' {
			cname += "."
		}
	}
	for r := res; r != nil; r = r.ai_next {
		// We only asked for SOCK_STREAM, but check anyhow.
		if r.ai_socktype != C.SOCK_STREAM {
			continue
		}
		switch r.ai_family {
		case C.AF_INET:
			sa := (*syscall.RawSockaddrInet4)(unsafe.Pointer(r.ai_addr))
			addr := IPAddr{IP: copyIP(sa.Addr[:])}
			addrs = append(addrs, addr)
		case C.AF_INET6:
			sa := (*syscall.RawSockaddrInet6)(unsafe.Pointer(r.ai_addr))
			addr := IPAddr{IP: copyIP(sa.Addr[:]), Zone: zoneToString(int(sa.Scope_id))}
			addrs = append(addrs, addr)
		}
	}
	return addrs, cname, nil, true
}

func cgoLookupIP(name string) (addrs []IPAddr, err error, completed bool) {
	addrs, _, err, completed = cgoLookupIPCNAME(name)
	return
}

func cgoLookupCNAME(name string) (cname string, err error, completed bool) {
	_, cname, err, completed = cgoLookupIPCNAME(name)
	return
}

func copyIP(x IP) IP {
	if len(x) < 16 {
		return x.To16()
	}
	y := make(IP, len(x))
	copy(y, x)
	return y
}
