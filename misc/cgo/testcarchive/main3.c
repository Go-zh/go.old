// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test os/signal.Notify and os/signal.Reset.
// This is a lot like misc/cgo/testcshared/main5.c.

#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sched.h>

#include "libgo3.h"

static void die(const char* msg) {
	perror(msg);
	exit(EXIT_FAILURE);
}

static volatile sig_atomic_t sigioSeen;

static void ioHandler(int signo, siginfo_t* info, void* ctxt) {
	sigioSeen = 1;
}

int main(int argc, char** argv) {
	int verbose;
	struct sigaction sa;
	int i;

	verbose = argc > 2;
	setvbuf(stdout, NULL, _IONBF, 0);

	if (verbose) {
		printf("calling sigaction\n");
	}

	memset(&sa, 0, sizeof sa);
	sa.sa_sigaction = ioHandler;
	if (sigemptyset(&sa.sa_mask) < 0) {
		die("sigemptyset");
	}
	sa.sa_flags = SA_SIGINFO;
	if (sigaction(SIGIO, &sa, NULL) < 0) {
		die("sigaction");
	}

	// At this point there should not be a Go signal handler
	// installed for SIGIO.

	if (verbose) {
		printf("raising SIGIO\n");
	}

	if (raise(SIGIO) < 0) {
		die("raise");
	}

	if (verbose) {
		printf("waiting for sigioSeen\n");
	}

	// Wait until the signal has been delivered.
	i = 0;
	while (!sigioSeen) {
		if (sched_yield() < 0) {
			perror("sched_yield");
		}
		i++;
		if (i > 100000) {
			fprintf(stderr, "looping too long waiting for signal\n");
			exit(EXIT_FAILURE);
		}
	}

	sigioSeen = 0;

	// Tell the Go code to catch SIGIO.

	if (verbose) {
		printf("calling CatchSIGIO\n");
	}

	CatchSIGIO();

	if (verbose) {
		printf("raising SIGIO\n");
	}

	if (raise(SIGIO) < 0) {
		die("raise");
	}

	if (verbose) {
		printf("calling SawSIGIO\n");
	}

	if (!SawSIGIO()) {
		fprintf(stderr, "Go handler did not see SIGIO\n");
		exit(EXIT_FAILURE);
	}

	if (sigioSeen != 0) {
		fprintf(stderr, "C handler saw SIGIO when only Go handler should have\n");
		exit(EXIT_FAILURE);
	}

	// Tell the Go code to stop catching SIGIO.

	if (verbose) {
		printf("calling ResetSIGIO\n");
	}

	ResetSIGIO();

	if (verbose) {
		printf("raising SIGIO\n");
	}

	if (raise(SIGIO) < 0) {
		die("raise");
	}

	if (verbose) {
		printf("calling SawSIGIO\n");
	}

	if (SawSIGIO()) {
		fprintf(stderr, "Go handler saw SIGIO after Reset\n");
		exit(EXIT_FAILURE);
	}

	if (verbose) {
		printf("waiting for sigioSeen\n");
	}

	// Wait until the signal has been delivered.
	i = 0;
	while (!sigioSeen) {
		if (sched_yield() < 0) {
			perror("sched_yield");
		}
		i++;
		if (i > 100000) {
			fprintf(stderr, "looping too long waiting for signal\n");
			exit(EXIT_FAILURE);
		}
	}

	printf("PASS\n");
	return 0;
}
