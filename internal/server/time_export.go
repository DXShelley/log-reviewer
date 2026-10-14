package server

import (
	"archive/zip"
	"fmt"
	"io"
	"log-reviewer/internal/logs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

func (s *Server) timeExport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	report, err := s.getReport()
	if err != nil {
		fail(w, 409, err)
		return
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	mode := q.Get("mode")
	if mode != "merged" && mode != "files" {
		fail(w, 400, fmt.Errorf("mode must be merged or files"))
		return
	}
	source := -1
	if v := q.Get("source"); v != "" {
		source, err = strconv.Atoi(v)
		if err != nil || source < -1 || source >= len(report.Sources) {
			fail(w, 400, fmt.Errorf("invalid source"))
			return
		}
	}
	tmp, err := os.CreateTemp(s.dir, "time-export-*")
	if err != nil {
		fail(w, 500, err)
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	var z *zip.Writer
	if mode == "files" {
		z = zip.NewWriter(tmp)
	}
	safe := regexp.MustCompile(`[^\pL\pN._-]+`)
	for i, src := range report.Sources {
		if source >= 0 && i != source {
			continue
		}
		var out io.Writer = tmp
		if z == nil {
			if st, e := tmp.Stat(); e != nil {
				err = e
			} else if st.Size() > 0 {
				var last [1]byte
				if _, err = tmp.ReadAt(last[:], st.Size()-1); err == nil && last[0] != '\n' {
					_, err = tmp.Write([]byte{'\n'})
				}
			}
		}
		if z != nil {
			name := src.Path
			if src.Remote != nil {
				name = src.Remote.Path
			}
			out, err = z.Create(fmt.Sprintf("%03d-%s-%s-%s.log", i+1, safe.ReplaceAllString(filepath.Base(name), "_"), safe.ReplaceAllString(from, "_"), safe.ReplaceAllString(to, "_")))
		}
		if err == nil {
			err = logs.ExportTime(r.Context(), report, i, from, to, out)
		}
		if err != nil {
			break
		}
	}
	if z != nil {
		closeErr := z.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		fail(w, 400, err)
		return
	}
	tmp.Seek(0, 0)
	name := "log-reviewer-time.txt"
	mime := "text/plain; charset=utf-8"
	if z != nil {
		name = "log-reviewer-time.zip"
		mime = "application/zip"
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	w.Header().Set("Content-Type", mime)
	if st, e := tmp.Stat(); e == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	}
	io.Copy(w, tmp)
}
