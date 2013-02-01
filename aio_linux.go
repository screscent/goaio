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
	"unsafe"
)

var ctx C.io_context_t
var max_event_size int
var cbs []C.struct_iocb
var pagesize C.size_t

type aio_result struct {
	buf  []byte
	size int
	err  error
}

var idle_event chan uint

var aio_result_map map[uint]chan aio_result

func InitAio(size int) error {
	max_event_size = size
	cbs = make([]C.struct_iocb, max_event_size, max_event_size)
	aio_result_map = make(map[uint]chan aio_result)
	idle_event = make(chan uint, max_event_size)
	for i := 0; i < max_event_size; i++ {
		idle_event <- uint(i)
		retch := make(chan aio_result, 1)
		aio_result_map[uint(i)] = retch
	}
	pagesize = C.get_pagesize()

	ret := C.io_setup(C.int(max_event_size), &ctx)
	if int(ret) != 0 {
		return errors.New("init aio failed")
	}
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

	rt := C.io_submit(ctx, 1, &cb)
	if int(rt) < 0 {
		return nil, errors.New("io submit failed")
	}

	ret := <-retch
	return ret.buf, ret.err
}

func WriteAt(fd int, off int64, buf []byte, size int) (int, error) {
	idx := <-idle_event
	fmt.Println("idx", idx)
	retch := aio_result_map[idx]
	defer func() {idle_event <- idx	}()

	var cb *C.struct_iocb = &cbs[idx]

	var write_buf unsafe.Pointer
	C.posix_memalign(&write_buf, pagesize, C.size_t(size))
	defer C.free(write_buf)
	for i := 0; i < size; i++ {
		*(*byte)(unsafe.Pointer(uintptr(write_buf) + uintptr(i))) = buf[i]
	}

	C.io_prep_pwrite(cb, C.int(fd), write_buf, C.size_t(size), C.longlong(off))
	cbs[idx].data = unsafe.Pointer(&idx)

	rt := C.io_submit(ctx, 1, &cb)
	if int(rt) < 0 {
		return 0, errors.New("io submit failed")
	}
	ret := <-retch
	return ret.size, ret.err
}

func run() {
	events := make([]C.struct_io_event, max_event_size, max_event_size)
	var time_out C.struct_timespec = C.struct_timespec{0, 0}

	for {
		n := C.io_getevents(ctx, C.long(1), C.long(max_event_size), &events[0], &time_out)
		for i := 0; i < int(n); i++ {

			func() {
				var cb *C.struct_iocb = events[i].obj
				iic := C.get_iic_from_iocb(cb)

				key := *(*uint)(unsafe.Pointer(cb.data))
				retch := aio_result_map[key]

				if C.int(events[i].res2) != 0 {
					retch <- aio_result{nil, 0, errors.New("aio error")}
					return
				}

				switch (*cb).aio_lio_opcode {

				case C.IO_CMD_PREAD:
					buf := C.GoBytes(unsafe.Pointer((*iic).buf), C.int(events[i].res))
					retch <- aio_result{buf, int(events[i].res), nil}

				case C.IO_CMD_PWRITE:
					retch <- aio_result{nil, int(events[i].res), nil}

				default:
					retch <- aio_result{nil, 0, errors.New("unk type")}

				}
			}()

		}
	}
}
