//author gonghh
//Copyright 2013 gonghh(screscent). Under the Apache License, Version 2.0.
package goaio

/*
#cgo LDFLAGS: -laio
#include <string.h>
#include <stdlib.h>
#include <libaio.h>
#include <unistd.h>
size_t get_pagesize()
{
	return sysconf(_SC_PAGESIZE);
}
struct io_iocb_common* get_iic_from_iocb(struct iocb* cb)
{
	return &(cb->u.c);
}
*/
import "C"

import (
	"errors"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

var ctx C.io_context_t
var max_event_size int
var cbs []C.struct_iocb
var pagesize C.size_t
var aio_lock *sync.Mutex
var aiocount int
var have_aio_event chan int

func init() {
	aio_lock = &sync.Mutex{}
	aiocount = 0
	have_aio_event = make(chan int)
}

type aio_result struct {
	buf  []byte
	size int
	err  error
}

var idle_event chan uint

var aio_result_map [](chan aio_result)

func InitAio(size int) error {
	max_event_size = size
	cbs = make([]C.struct_iocb, max_event_size, max_event_size)
	aio_result_map = make([](chan aio_result), max_event_size, max_event_size)
	idle_event = make(chan uint, max_event_size)
	for i := 0; i < max_event_size; i++ {
		idle_event <- uint(i)
		retch := make(chan aio_result, 1)
		aio_result_map[i] = retch
	}
	pagesize = C.get_pagesize()
	aio_lock.Lock()
	ret := C.io_setup(C.int(max_event_size), &ctx)
	if int(ret) != 0 {
		aio_lock.Unlock()
		return errors.New("init aio failed")
	}
	aio_lock.Unlock()

	go run()

	return nil
}

func ReadAt(fd int, off int64, size int) ([]byte, error) {
	idx := <-idle_event
	retch := aio_result_map[idx]
	defer func() { idle_event <- idx }()

	var cb *C.struct_iocb = &cbs[idx]

	var read_buf unsafe.Pointer
	C.posix_memalign(&read_buf, pagesize, C.size_t(size))
	defer C.free(read_buf)

	C.io_prep_pread(cb, C.int(fd), read_buf, C.size_t(size), C.longlong(off))
	cbs[idx].data = unsafe.Pointer(&idx)

	aio_lock.Lock()
	rt := C.io_submit(ctx, 1, &cb)
	if int(rt) < 0 {
		aio_lock.Unlock()
		return nil, errors.New("io submit failed")
	}
	aiocount++
	aio_lock.Unlock()

	select {
	case have_aio_event <- 0:
	default:
	}

	ret := <-retch

	return ret.buf, ret.err
}

func WriteAt(fd int, off int64, buf []byte) (int, error) {
	size := len(buf)
	if size == 0 {
		return 0, nil
	}

	idx := <-idle_event
	retch := aio_result_map[idx]
	defer func() { idle_event <- idx }()

	var cb *C.struct_iocb = &cbs[idx]

	var write_buf unsafe.Pointer
	C.posix_memalign(&write_buf, pagesize, C.size_t(size))
	defer C.free(write_buf)
	for i := 0; i < size; i++ {
		*(*byte)(unsafe.Pointer(uintptr(write_buf) + uintptr(i))) = buf[i]
	}

	C.io_prep_pwrite(cb, C.int(fd), write_buf, C.size_t(size), C.longlong(off))
	cbs[idx].data = unsafe.Pointer(&idx)

	aio_lock.Lock()
	rt := C.io_submit(ctx, 1, &cb)

	if int(rt) < 0 {
		aio_lock.Unlock()
		return 0, errors.New("io submit failed")
	}
	aiocount++
	aio_lock.Unlock()

	select {
	case have_aio_event <- 0:
	default:
	}

	ret := <-retch
	return ret.size, ret.err
}

func run() {
	runtime.LockOSThread()

	events := make([]C.struct_io_event, max_event_size, max_event_size)
	var time_out C.struct_timespec = C.struct_timespec{0, 0}
	tick := time.Tick(1 * time.Microsecond)
	for {

		select {
		case <-have_aio_event:
		case <-tick:
		}

		aio_lock.Lock()
		if aiocount == 0 {
			aio_lock.Unlock()
			continue
		}
		n := C.io_getevents(ctx, C.long(1), C.long(max_event_size), &events[0], &time_out)
		if n > 0 {
			aiocount -= int(n)
		}
		aio_lock.Unlock()
		if n <= 0 {
			continue
		}
		wg := &sync.WaitGroup{}
		wg.Add(int(n))

		for i := 0; i < int(n); i++ {

			go func(idx int) {
				defer wg.Done()

				var cb *C.struct_iocb = events[idx].obj
				iic := C.get_iic_from_iocb(cb)

				key := *(*uint)(unsafe.Pointer(cb.data))

				retch := aio_result_map[key]

				if C.int(events[idx].res2) != 0 {
					retch <- aio_result{nil, 0, errors.New("aio error")}
					return
				}

				switch (*cb).aio_lio_opcode {

				case C.IO_CMD_PREAD:
					buf := C.GoBytes(unsafe.Pointer((*iic).buf), C.int(events[idx].res))
					retch <- aio_result{buf, int(events[idx].res), nil}

				case C.IO_CMD_PWRITE:
					retch <- aio_result{nil, int(events[idx].res), nil}

				default:
					retch <- aio_result{nil, 0, errors.New("unk type")}

				}
			}(i)
		}

		wg.Wait()
	}
}
