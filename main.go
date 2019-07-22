package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

func main() {
	// pid := flag.Uint64("p", 0, "process id")
	interval := flag.Duration("i", 1*time.Second, "interval between status checks")
	length := flag.Duration("l", 5*time.Millisecond, "length of time to run")
	format := flag.String("f", "human", "output format (human, csv, json)")
	unitf := flag.String("u", "", "unit such as 'MiB' or 'kB'. Default is best fit")
	flag.Parse()

	pidargs := flag.Args()
	if len(pidargs) == 0 {
		flag.Usage()
		// flag.PrintDefaults()
		return
	}

	pids := make([]uint64, len(pidargs))
	for i := range pids {
		pid, err := strconv.Atoi(pidargs[i])
		if err != nil {
			fmt.Printf("%s is not a valid process id\n", pidargs[i])
			return
		}
		pids[i] = uint64(pid)
	}

	var style formatter
	if f, ok := formats[*format]; ok {
		style = f
	} else {
		fmt.Printf("invalid format %s.\n", *format)
		return
	}

	unit, err := ParseUnit(*unitf)
	if err != nil {
		fmt.Printf("invalid unit %s\n", *unitf)
		return
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Kill, os.Interrupt)
	done := make(chan struct{})

	go func() {
		var maxSize, maxRes Size
		timer := time.After(0 * time.Second)
		timeout := time.After(*length)
		var o output

		fmt.Println(style(o, Default)) // print header
		for {
			select {
			case <-timeout:
				close(done)
				return

			case <-sig:
				close(done)
				return

			case <-timer:
				for _, pid := range pids {
					stat, err := Statm(pid)
					if err != nil {
						fmt.Println(err)
						close(done)
						return
					}
					if stat[size] > maxSize {
						maxSize = stat[size]
					}
					if stat[resident] > maxRes {
						maxRes = stat[resident]
					}
					o.when = time.Now()
					o.pid = pid
					o.peakSize = maxSize
					o.currentSize = stat[size]
					o.peakResident = maxRes
					o.currentResident = stat[resident]

					fmt.Println(style(o, unit))
				}

				timer = time.After(*interval)
			}
		}
	}()

	<-done
}

type output struct {
	when                          time.Time
	pid                           uint64
	peakSize, currentSize         Size
	peakResident, currentResident Size
}

type formatter func(o output, unit Size) string

func human(o output, unit Size) string {
	const f = "%-28s %-15s %-15s %-15s %-15s %-15s"
	zero := output{}
	if o == zero {
		return fmt.Sprintf(f, "time", "pid", "peak_size", "current_size", "peak_resident", "current_resident")
	}
	return fmt.Sprintf(f,
		o.when.Format(time.RFC3339), strconv.FormatUint(o.pid, 10),
		o.peakSize.InString(unit), o.currentSize.InString(unit),
		o.peakResident.InString(unit), o.currentResident.InString(unit))
}

func csv(o output, unit Size) string {
	const f = "%s,%s,%s,%s,%s,%s"
	zero := output{}
	if o == zero {
		return fmt.Sprintf(f, "time", "pid", "peak_size", "current_size", "peak_resident", "current_resident")
	}
	return fmt.Sprintf(f,
		o.when.Format(time.RFC3339), strconv.FormatUint(o.pid, 10),
		o.peakSize.InString(unit), o.currentSize.InString(unit),
		o.peakResident.InString(unit), o.currentResident.InString(unit))
}

func json(o output, unit Size) string {
	var f string
	if unit == Default {
		f = `{"time": "%s", "pid": %s, "peak_size": "%s", "current_size": "%s", "peak_resident": "%s", "current_resident": "%s"}`
	} else {
		f = `{"time": "%s", "pid": %s, "peak_size": %s, "current_size": %s, "peak_resident": %s, "current_resident": %s}`
	}
	zero := output{}
	if o == zero {
		// no "header" for json
		// return fmt.Sprintf(f, "time", "pid", "peak_size", "current_size", "peak_resident", "current_resident")
		return ""
	}
	return fmt.Sprintf(f,
		o.when.Format(time.RFC3339), strconv.FormatUint(o.pid, 10),
		o.peakSize.InString(unit), o.currentSize.InString(unit),
		o.peakResident.InString(unit), o.currentResident.InString(unit))

}

var formats = map[string]formatter{
	"human": human,
	"csv":   csv,
	"json":  json,
}

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

type Size uint64

// Size constants 0-6
const (
	bite Size = iota // 1<<(10*0)
	kilo             // 1<<(10*1) etc...
	mega
	giga
	tera
	peta
	exta
)

// Size units
const (
	B Size = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
	PiB
	EiB
	Default = 0
)

var suffix = [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

func u(c Size) Size { return 1 << (10 * c) }

func (s Size) String() string {
	for i := bite; i < exta; i++ {
		fit := s / u(i+1)
		if fit == 0 {
			return fmt.Sprintf("%.4g%s", float64(s)/float64(u(i)), suffix[i])
		}
	}
	return fmt.Sprintf("%d%s", s, exta)
}

func (s Size) In(unit Size) float64 {
	return float64(s) / float64(unit)
}

func (s Size) InString(unit Size) string {
	if unit == 0 {
		return s.String()
	}

	// do some trickery so that when the converted
	// size has all zeros after the decimal place,
	// it will print as an integer instead of with
	// the trailing zeros. If the decimal places are
	// important, print the size as a float.
	in := s.In(unit)
	if float64(s/unit) == in {
		return fmt.Sprintf("%d", int64(in))
	}
	return fmt.Sprintf("%.3f", in)

}

func ParseUnit(unit string) (Size, error) {
	s := strings.ToLower(unit)
	switch s {
	case "":
		return Default, nil
	case "b":
		return B, nil
	case "kb", "kib":
		return KiB, nil
	case "mb", "mib":
		return MiB, nil
	case "gb", "gib":
		return GiB, nil
	case "tb", "tib":
		return TiB, nil
	case "pb", "pib":
		return PiB, nil
	case "eb", "eib":
		return EiB, nil
	}

	return 0, fmt.Errorf("%s invalid size", unit)
}

func ParseSize(s string) (Size, error) {
	suf := strings.TrimSpace(strings.TrimLeft(s, "-0123456789"))
	num := strings.TrimSpace(strings.TrimSuffix(s, suf))
	n, err := strconv.ParseUint(num, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeral \"%s\" in %s", num, s)
	}
	unit, err := ParseUnit(suf)
	if err != nil {
		return 0, fmt.Errorf("invalid unit in %s: %s", s, err)
	}
	return Size(n) * unit, nil
}
