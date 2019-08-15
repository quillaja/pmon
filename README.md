# pmon
Monitors memory usage for linux processes.

Essentially it reads `/proc/<pid>/mstat`, extracts the 'size' and 'resident', tracks maximum values, and prints this data to standard out each interval. Various user options exist to change the interval, run-time length, and output format. It can operate on multiple PIDs at once.

    Usage: pmon [FLAG]... [PID]...
    Monitor process memory and outputs in various formats.

    -cmd string
            runs the command and monitors it. SIGKILL is sent when pmon exits.
            cmd's stdout is sent to /dev/null
    -f string
            output format (human, csv, json) (default "human")
    -graph
            show a graph of the memory useage when done.
    -i duration
            interval between status checks (default 1s)
    -l duration
            length of time to run (default 5ms)
    -u string
            unit such as 'MiB' or 'kB'. (default is best fit)

    Examples:

    pmon -l 5m -u kb 8231
        monitors 8231 for 5 mins showing memory in KiB.
    pmon -l 1h30m5s -i 5s -f csv -cmd "sleep 10" 8231
        runs 'sleep' and monitors it and 8231 for 1 hour 30 mins 5 sec
        with a 5 sec interval and formatting to CSV.
