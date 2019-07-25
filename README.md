# pmon
Monitors memory usage for linux processes.

Essentially it reads `/proc/<pid>/mstat`, extracts the 'size' and 'resident', tracks maximum values, and prints this data to standard out each interval. Various user options exist to change the interval, run-time length, and output format. It can operate on multiple PIDs at once.
