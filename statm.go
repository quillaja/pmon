package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

type statm [7]Size

// size, resident, shared, text, lib, data, dt uint64
const (
	size = iota
	resident
	shared
	text
	lib
	data
	dt
)

func Statm(pid uint64) (mem statm, err error) {
	const pathstatm = "/proc/%d/statm"

	fname := fmt.Sprintf(pathstatm, pid)
	raw, err := ioutil.ReadFile(fname)
	if err != nil {
		err = fmt.Errorf("no such pid (%d)", pid)
		return
	}

	_, err = fmt.Sscan(string(raw), &mem[size], &mem[resident], &mem[shared],
		&mem[text], &mem[lib], &mem[data], &mem[dt])
	if err != nil {
		err = fmt.Errorf("couldn't scan statm data: %s", err)
		return
	}

	// convert pages to bytes
	page := Size(os.Getpagesize()) // page size in bytes
	for i := size; i <= dt; i++ {
		mem[i] *= page
	}

	return mem, nil
}
