// Generates real (non-sparse) files and measures streaming classification.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log-reviewer/internal/logs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	dir := flag.String("dir", "", "benchmark directory with sufficient free space")
	gib := flag.Int("gib", 1, "file size in GiB")
	dense := flag.Bool("dense", false, "all ERROR stress workload")
	flag.Parse()
	if *dir == "" || *gib < 1 {
		fmt.Fprintln(os.Stderr, "--dir and --gib >= 1 required")
		os.Exit(1)
	}
	os.MkdirAll(*dir, 0700)
	name := fmt.Sprintf("bench-%dgib", *gib)
	if *dense {
		name += "-dense"
	}
	path := filepath.Join(*dir, name+".log")
	pattern := ""
	for i := 0; i < 100; i++ {
		level := "INFO"
		message := "request completed service=orders route=/api/orders latency=15ms status=200 trace=0000000000000001"
		if *dense || i%10 == 0 {
			level = "ERROR"
			message = "database connection failed service=orders host=db.internal code=500"
		} else if i%25 == 0 {
			level = "WARN"
			message = "request slow service=orders route=/api/orders latency=900ms"
		}
		pattern += "2026-10-14 12:30:00.123 " + level + " 1234 --- [worker-1] com.demo.OrderService : " + message + "\n"
	}
	chunk := strings.Repeat(pattern, 64)
	f, err := os.Create(path)
	check(err)
	w := bufio.NewWriterSize(f, 1024*1024)
	remaining := int64(*gib) * 1024 * 1024 * 1024
	for remaining > 0 {
		n := int64(len(chunk))
		if n > remaining {
			n = remaining
		}
		_, err = w.WriteString(chunk[:n])
		check(err)
		remaining -= n
	}
	check(w.Flush())
	check(f.Close())
	fmt.Fprintln(os.Stderr, "generated", path)
	start := time.Now()
	r, err := logs.Analyze([]string{path}, *dir, logs.Options{})
	check(err)
	wall := time.Since(start)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	out := map[string]any{"file": path, "bytes": r.Bytes, "elapsed_ms": r.ElapsedMS, "wall_ms": wall.Milliseconds(), "mib_per_second": float64(r.Bytes) / 1048576 / wall.Seconds(), "events": r.Events, "categories": len(r.Groups), "heap_mib": float64(mem.HeapAlloc) / 1048576, "runtime_sys_mib": float64(mem.Sys) / 1048576, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "dense": *dense}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
	check(os.WriteFile(filepath.Join(*dir, name+".json"), b, 0600))
	check(logs.SaveReport(r, filepath.Join(*dir, name+"-report.json")))
}
func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
