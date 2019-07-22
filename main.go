package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
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
