package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/zserge/lorca"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	interval := flag.Duration("i", 1*time.Second, "interval between status checks")
	length := flag.Duration("l", 5*time.Millisecond, "length of time to run")
	format := flag.String("f", "human", "output format (human, csv, json)")
	unitf := flag.String("u", "", "unit such as 'MiB' or 'kB'. (default is best fit)")
	graphf := flag.Bool("graph", false, "show a graph of the memory useage when done.")
	cmdf := flag.String("cmd", "", "runs the command and monitors it. SIGKILL is sent when pmon exits.\ncmd's stdout is sent to /dev/null")
	flag.Parse()

	flag.Usage = func() {
		fmt.Println("Usage: pmon [FLAG]... [PID]...\nMonitor process memory and outputs in various formats.\n")
		flag.PrintDefaults()
		fmt.Println("\nExamples:\n\n  pmon -l 5m -u kb 8231\n\tmonitors 8231 for 5 mins showing memory in KiB.")
		fmt.Println("  pmon -l 1h30m5s -i 5s -f csv -cmd \"sleep 10\" 8231\n\truns 'sleep' and monitors it and 8231 for 1 hour 30 mins 5 sec\n\twith a 5 sec interval and formatting to CSV.")
	}

	pidargs := flag.Args()
	if len(pidargs) == 0 && *cmdf == "" {
		flag.Usage()
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

	if *cmdf != "" {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		parts := strings.Split(*cmdf, " ")
		cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
		err := cmd.Start()
		if err != nil {
			fmt.Printf("error with command: %s\n", err)
			return
		}
		pids = append(pids, uint64(cmd.Process.Pid))
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
	// var file string
	// defer file.Close()
	if *graphf {
		// file, err = os.Create(*pngf)
		// file = *pngf
		// _, err = os.Stat(file)
		// if err != nil {
		// 	fmt.Printf("couldn't open png: %s", err)
		// 	return
		// }
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
				if len(pids) == 0 {
					close(done)
					return
				}
				toDelete := map[uint64]bool{}

				for _, pid := range pids {
					stat, err := Statm(pid)
					if err != nil {
						// if pid gives an error, print err to stderr
						// and mark pid for deletion
						fmt.Fprint(os.Stderr, err)
						toDelete[pid] = true
						continue
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

				// remove pids marked for deletion
				if len(toDelete) > 0 {
					old := pids
					pids = []uint64{}
					for _, pid := range old {
						if del := toDelete[pid]; !del {
							pids = append(pids, pid)
						}
					}
				}

				timer = time.After(*interval)
			}
		}
	}()

	<-done

	if graphing {
		err = hist.graph()
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

func (h *history) graph() error {

	// try lorca for chrome window and plotly.js
	// this actually isn't too bad
	type ctx struct {
		Pid   uint64
		Times []time.Time
		Rss   []float64
	}
	thedata := []ctx{}
	for pid := range h.times {
		thedata = append(thedata, ctx{
			pid,
			h.times[pid],
			h.rss[pid]})
	}

	html := `
	<html>
		<head>
		<title>pmon</title>
		<!-- Plotly.js -->
		<script src="https://cdn.plot.ly/plotly-latest.min.js"></script>
		</head>
		<body><div id="graph"></div></body>
		<script>
		let data = [
			{{ range . }}
			{
				x: [{{ range .Times }}"{{.Format "2006-01-02 15:04:05"}}",{{ end }}],
				y: [{{ range .Rss }}{{.}},{{ end }}],
				name: {{.Pid}},
				mode: "scatter",
			},
			{{ end }}
		];

		let layout = {
			autosize: true,
			width: 780,
			height: 580,
			title: "` + fmt.Sprintf("RSS (%s)", h.unit.Suffix()) + `"
		};

		Plotly.newPlot('graph', data, layout);
		</script>
	</html>
	`
	tpl, err := template.New("plot").Parse(html)
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, thedata)
	if err != nil {
		return err
	}
	page := buf.String()
	// fmt.Println(page)

	ui, err := lorca.New("data:text/html,"+url.PathEscape(page), "", 800, 600)
	if err != nil {
		return err
	}
	defer ui.Close()
	// Wait until UI window is closed
	<-ui.Done()

	return nil
}

type output struct {
	when                          time.Time
	pid                           uint64
	peakSize, currentSize         Size
	peakResident, currentResident Size
}

const timefmt = "2006-01-02 15:04:05"

type formatter func(o output, unit Size) string

func human(o output, unit Size) string {
	const f = "%-20s %-15s %-15s %-15s %-15s %-15s"
	zero := output{}
	if o == zero {
		return fmt.Sprintf(f, "time", "pid", "peak_size", "current_size", "peak_resident", "current_resident")
	}
	return fmt.Sprintf(f,
		o.when.Format(timefmt), strconv.FormatUint(o.pid, 10),
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
		o.when.Format(timefmt), strconv.FormatUint(o.pid, 10),
		o.peakSize.InString(unit), o.currentSize.InString(unit),
		o.peakResident.InString(unit), o.currentResident.InString(unit))
}

func json(o output, unit Size) string {
	var f string
	if unit == Default {
		f = `{"time": "%s", "pid": %s, "peak_size": "%s", "current_size": "%s", "peak_resident": "%s", "current_resident": "%s"},`
	} else {
		f = `{"time": "%s", "pid": %s, "peak_size": %s, "current_size": %s, "peak_resident": %s, "current_resident": %s},`
	}
	zero := output{}
	if o == zero {
		// no "header" for json
		// return fmt.Sprintf(f, "time", "pid", "peak_size", "current_size", "peak_resident", "current_resident")
		return ""
	}
	return fmt.Sprintf(f,
		o.when.Format(timefmt), strconv.FormatUint(o.pid, 10),
		o.peakSize.InString(unit), o.currentSize.InString(unit),
		o.peakResident.InString(unit), o.currentResident.InString(unit))

}

var formats = map[string]formatter{
	"human": human,
	"csv":   csv,
	"json":  json,
}
