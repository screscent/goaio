# goaio
=====

goaio

# License
===
author gonghh   
Copyright 2013 gonghh(screscent).  
Under the [Apache License, Version 2.0](http://www.apache.org/licenses/LICENSE-2.0.html).  

# example
===

```go  

package main

import (
	"errors"
	"fmt"
	"goaio"
	"os"
	"runtime"
	"sync"
	"syscall"
)

func open(filepath string, length int64) (err error) {
	st, err := os.Stat(filepath)
	var fd *os.File
	if err != nil && os.IsNotExist(err) {
		fd, err = os.Create(filepath)
		defer fd.Close()
		if err != nil {
			return
		}
	} else {
		fd, err = os.OpenFile(filepath, os.O_RDWR, 0600)
		defer fd.Close()
		if st.Size() == length {
			return
		}
	}

	if length > 0 {
		err = os.Truncate(filepath, length)
		if err != nil {
			return
			err = errors.New("Could not truncate file.")
		}
	}

	return
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	open("./test.file", 1024*1000*1000)
	//must O_DIRECT
	fd, err := syscall.Open("./test.file", syscall.O_DIRECT|syscall.O_NONBLOCK|syscall.O_RDWR, 0600)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer syscall.Close(fd)
	goaio.InitAio(100)
	buf := make([]byte, 0, 2000)
	for i := 0; i < 1024; {
		for k := 0; k < 10 && i < 1024; k++ {
			buf = append(buf, byte('0')+byte(k))
			i++
		}
	}
	buf2 := make([]byte, 1024, 1024)
	for i := 0; i < 1024; i++ {
		buf2[i] = 'a'
	}
	wg := &sync.WaitGroup{}
	wg.Add(1024)

	for i := 0; i < 1024; i += 2 {
		go func(idx int) {
			fmt.Println(idx)
			goaio.WriteAt(fd, int64(idx*1024), buf[0:1024])
			wg.Done()

		}(i)

		go func(idx int) {
			fmt.Println(idx + 1)
			goaio.WriteAt(fd, int64((idx+1)*1024), buf2)
			wg.Done()

		}(i)

	}
	wg.Wait()
	fmt.Println("start read....")

	wg = &sync.WaitGroup{}
	wg.Add(1024)
	for i := 0; i < 1024; i += 2 {
		go func(idx int) {
			_, _ = goaio.ReadAt(fd, int64(idx*1024), 1024)
			fmt.Println(idx)
			wg.Done()
		}(i)

		go func(idx int) {
			_, _ = goaio.ReadAt(fd, int64((idx+1)*1024), 1024)
			fmt.Println(idx + 1)
			wg.Done()
		}(i)
	}

	wg.Wait()
	fmt.Println("read end....")

}



```
