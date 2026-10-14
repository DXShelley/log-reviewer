package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log-reviewer/internal/logs"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
)

func exportTime(args []string) error {
	f := flag.NewFlagSet("export-time", flag.ContinueOnError)
	reportPath := f.String("report", "report.json", "analysis report")
	from := f.String("from", "", "start time, required")
	to := f.String("to", "", "inclusive end time, required")
	out := f.String("out", "time-logs.txt", "merged output, must not exist")
	dir := f.String("out-dir", "", "write one file per source into a new directory")
	source := f.Int("source", -1, "source index, -1 for all")
	if err := f.Parse(args); err != nil {
		return err
	}
	r, err := logs.LoadReport(*reportPath)
	if err != nil {
		return err
	}
	if *source < -1 || *source >= len(r.Sources) {
		return fmt.Errorf("invalid source")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	stage, err := os.MkdirTemp("", "log-reviewer-export-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	safe := regexp.MustCompile(`[^\pL\pN._-]+`)
	names := []string{}
	var merged *os.File
	if *dir == "" {
		merged, err = os.Create(filepath.Join(stage, "merged"))
		if err != nil {
			return err
		}
		defer merged.Close()
	}
	for i, s := range r.Sources {
		if *source >= 0 && i != *source {
			continue
		}
		name := s.Path
		if s.Remote != nil {
			name = s.Remote.Path
		}
		name = fmt.Sprintf("%03d-%s-%s-%s.log", i+1, safe.ReplaceAllString(filepath.Base(name), "_"), safe.ReplaceAllString(*from, "_"), safe.ReplaceAllString(*to, "_"))
		file := merged
		if merged != nil {
			st, e := merged.Stat()
			if e != nil {
				return e
			}
			if st.Size() > 0 {
				var last [1]byte
				if _, e = merged.ReadAt(last[:], st.Size()-1); e != nil {
					return e
				}
				if last[0] != '\n' {
					if _, e = merged.Write([]byte{'\n'}); e != nil {
						return e
					}
				}
			}
		}
		if file == nil {
			file, err = os.Create(filepath.Join(stage, name))
			if err != nil {
				return err
			}
			names = append(names, name)
		}
		err = logs.ExportTime(ctx, r, i, *from, *to, file)
		if merged == nil {
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			return err
		}
	}
	if merged != nil {
		if err = merged.Close(); err != nil {
			return err
		}
		return publishFile(filepath.Join(stage, "merged"), *out)
	}
	if err = os.Mkdir(*dir, 0700); err != nil {
		return err
	}
	completed := false
	created := []string{}
	defer func() {
		if !completed {
			for _, p := range created {
				os.Remove(p)
			}
			os.Remove(*dir)
		}
	}()
	for _, name := range names {
		if err = publishFile(filepath.Join(stage, name), filepath.Join(*dir, name)); err != nil {
			return err
		}
		created = append(created, filepath.Join(*dir, name))
	}
	completed = true
	return nil
}

func publishFile(source, dest string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dest)
	}
	return err
}
