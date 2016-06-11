// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include <pthread.h>
#include <signal.h>
#include <stdio.h>
#include <sys/limits.h>
#include "libcgo.h"

#define magic1 (0x23581321345589ULL)

// inittls allocates a thread-local storage slot for g.
//
// It finds the first available slot using pthread_key_create and uses
// it as the offset value for runtime.tlsg.
static void
inittls(void **tlsg, void **tlsbase)
{
	pthread_key_t k;
	int i, err;

	err = pthread_key_create(&k, nil);
	if(err != 0) {
		fatalf("pthread_key_create failed: %d", err);
	}
	pthread_setspecific(k, (void*)magic1);
	for (i=0; i<PTHREAD_KEYS_MAX; i++) {
		if (*(tlsbase+i) == (void*)magic1) {
			*tlsg = (void*)(i*sizeof(void *));
			pthread_setspecific(k, 0);
			return;
		}
	}
	fatalf("could not find pthread key");
}

void (*x_cgo_inittls)(void **tlsg, void **tlsbase) = inittls;
