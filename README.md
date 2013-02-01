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
	"syscall"
)



func main() {
  //must O_DIRECT
	fd, err := syscall.Open("./test.file", syscall.O_DIRECT|syscall.O_NONBLOCK|syscall.O_RDWR, 0600)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer syscall.Close(fd)
  
	goaio.InitAio(100)
  
	buf1 := make([]byte, 0, 2000)
	for i := 0; i < 1024; {
		for k := 0; k < 10 && i < 1024; k++ {
			buf1 = append(buf1, byte('0')+byte(k))
			i++
		}
	}
	buf2 := make([]byte, 1024, 1024)

	for i := 0; i < 1024; i += 2 {
		fmt.Println(i)
		goaio.WriteAt(fd, int64(i*1024), buf1, 1024)
		goaio.WriteAt(fd, int64((i+1)*1024), buf2, 1024)
	}

	for i := 0; i < 1024; i += 2 {
		fmt.Println(i)
		buf3, _ := goaio.ReadAt(fd, int64(i*1024), 1024)
		fmt.Println(i, buf3)
		buf4, _ := goaio.ReadAt(fd, int64((i+1)*1024), 1024)
		fmt.Println(i, buf4)
	}
}

```
