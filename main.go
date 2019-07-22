package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// pid := flag.Uint64("p", 0, "process id")
	interval := flag.Duration("i", 1*time.Second, "interval between status checks")
	length := flag.Duration("l", 5*time.Millisecond, "length of time to run")
	format := flag.String("f", "human", "output format (human, csv, json)")
	unitf := flag.String("u", "", "unit such as 'MiB' or 'kB'. Default is best fit")
	// pngf := flag.String("png", "", "filename of PNG in which to render a graph")
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

	var graphing bool
	var hist *history
	var file string
	// defer file.Close()
	if false { //*pngf != "" {
		// file, err = os.Create(*pngf)
		// file = *pngf
		_, err = os.Stat(file)
		if err != nil {
			fmt.Printf("couldn't open png: %s", err)
			return
		}
		hist = makeHistory(unit, pids...)
		if hist.unit == Default {
			// 'Default' will cause bad values. MiB is a reasonable default
			hist.unit = MiB
		}
		graphing = true
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

					if graphing {
						hist.add(o)
					}
				}

				timer = time.After(*interval)
			}
		}
	}()

	<-done

	if graphing {
		err = hist.graph(file)
		if err != nil {
			fmt.Println(err)
		}
	}
}

type history struct {
	unit     Size
	times    map[uint64][]time.Time
	rss      map[uint64][]float64
	min, max float64
	len      uint
}

func makeHistory(unit Size, pids ...uint64) *history {
	h := history{
		unit:  unit,
		times: make(map[uint64][]time.Time, len(pids)),
		rss:   make(map[uint64][]float64, len(pids)),
		max:   0,
	}

	for _, p := range pids {
		h.times[p] = make([]time.Time, 0)
		h.rss[p] = make([]float64, 0)
	}

	return &h
}

func (h *history) add(o output) {
	h.times[o.pid] = append(h.times[o.pid], o.when)
	end := o.currentResident.In(h.unit)
	h.rss[o.pid] = append(h.rss[o.pid], end)
	h.min = math.Min(h.min, end)
	h.max = math.Max(h.max, end)
}

func (h *history) graph(filename string) error {
	// sucks-----
	// lines := []chart.Series{}
	// for pid := range h.times {
	// 	lines = append(lines, chart.TimeSeries{
	// 		Name:    strconv.FormatUint(pid, 10),
	// 		XValues: h.times[pid],
	// 		YValues: h.rss[pid],
	// 	})
	// }

	// g := chart.Chart{
	// 	Series: lines,
	// 	XAxis:  chart.XAxis{Style: chart.StyleShow()},
	// }
	// g.YAxis.Name = "Resident Size (" + h.unit.String() + ")"
	// // manually create y
	// g.YAxis.Range = &chart.ContinuousRange{Min: 0, Max: h.max + 0.1*h.max}
	// return g.Render(chart.PNG, w)

	// also sucks -----
	// p, _ := plot.New()

	// p.Y.Label.Text = "Resident Size (" + h.unit.String() + ")"
	// p.X.Tick.Marker = plot.TimeTicks{Format: "2006-01-02\n15:04"}

	// for pid := range h.times {
	// 	pts := make(plotter.XYs, 0)
	// 	for i := range h.times[pid] {
	// 		pts = append(pts, plotter.XY{
	// 			X: float64(h.times[pid][i].Unix()),
	// 			Y: h.rss[pid][i]})
	// 	}
	// 	plotutil.AddLinePoints(p, strconv.FormatUint(pid, 10), pts)
	// }
	// return p.Save(800, 600, filename)

	// try another package
	// this one is easier to use, except the names of types are idiotic
	// plt.Reset(true, nil)
	// plt.SetYlabel("Resident Size ("+h.unit.String()+")", nil)
	// plt.Legend(&plt.A{LegLoc: "left", LegNcol: 2})

	// for pid := range h.times {
	// 	// make x's
	// 	x := []float64{}
	// 	for i := range h.times[pid] {
	// 		x = append(x, float64(i))
	// 	}
	// 	// make plot
	// 	plt.Plot(x, h.rss[pid], &plt.A{
	// 		L:      strconv.FormatUint(pid, 10),
	// 		C:      colorful.FastHappyColor().Hex(),
	// 		Closed: false})
	// }
	// // plt.Equal()
	// plt.Save(".", strings.TrimSuffix(filename, ".png"))

	return nil
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
