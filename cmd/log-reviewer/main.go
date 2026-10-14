package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/server"
)

const version = "0.6.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return gui(nil)
	}
	switch args[0] {
	case "gui", "serve":
		return gui(args[1:])
	case "analyze":
		return analyze(args[1:])
	case "fetch":
		return fetch(args[1:])
	case "export-time":
		return exportTime(args[1:])
	case "languages":
		r, err := logs.BuiltinLanguages()
		if err != nil {
			return err
		}
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		return e.Encode(r)
	case "query", "export":
		return query(args[0], args[1:])
	case "version", "--version":
		fmt.Println("log-reviewer", version)
		return nil
	case "help", "--help", "-h":
		fmt.Print(help)
		return nil
	default:
		return fmt.Errorf("unknown command %q; run log-reviewer help", args[0])
	}
}

const help = `Log Reviewer 0.6.0 - local, streaming log classification

  log-reviewer                              Open local GUI
  log-reviewer gui [--host 0.0.0.0] [--port 8765] [--no-browser] [files...]
    [--allowed-hosts logs.example.com] [--cache-dir directory] [--rule-dir directory]
  log-reviewer analyze [--out report.json] [--all] [--rules rules.json] files...
  log-reviewer languages                     Export built-in language rules as JSON
  log-reviewer fetch --host SERVER --user USER --path PATH --key PRIVATE_KEY
    [--fingerprint SHA256:...] [--known-hosts FILE] [--ssh-dir DIR] [--recursive] [--analyze]
    [--out-dir remote-logs] [--pattern *.log,*.txt] [--max-files 1000] [--max-bytes 10737418240]
  log-reviewer analyze --language-rules languages.json [files...]
  log-reviewer query --report report.json [filters] [--details] [--offset 0] [--limit 20]
  log-reviewer export --report report.json [filters] [--out errors.txt]
  log-reviewer export-time --report report.json --from TIME --to TIME
    [--out time-logs.txt] [--out-dir NEW_DIRECTORY] [--source N]

Filters: --group ID --level ERROR|WARN|FATAL --keyword TEXT --regex PATTERN
         --from RFC3339 --to RFC3339 --source N (zero-based file index)
Context: --before 20 --after 100 (0-10000 lines, with query --details)
All flags must precede positional file paths. Directory inputs include *.log, *.log.* and *.txt.
Query emits JSON; export streams full occurrences. Regex matches physical lines.
Local analysis requires no network or external runtime. Optional fetch uses SFTP.
`

func rules(path string) ([]logs.Rule, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r []logs.Rule
	err = json.Unmarshal(b, &r)
	return r, err
}
func analyze(args []string) error {
	f := flag.NewFlagSet("analyze", flag.ContinueOnError)
	out := f.String("out", "report.json", "report path")
	all := f.Bool("all", false, "include INFO/DEBUG/TRACE")
	rulePath := f.String("rules", "", "custom rule file")
	languagePath := f.String("language-rules", "", "language rule overrides JSON array")
	if err := f.Parse(args); err != nil {
		return err
	}
	r, err := rules(*rulePath)
	if err != nil {
		return err
	}
	var languages []logs.LanguageRule
	ruleDirectory, err := logs.DefaultRuleDirectory()
	if err != nil {
		return err
	}
	languages, err = (&logs.RuleStore{Dir: ruleDirectory}).Overrides()
	if err != nil {
		return err
	}
	if *languagePath != "" {
		b, err := os.ReadFile(*languagePath)
		if err != nil {
			return err
		}
		var overrides []logs.LanguageRule
		if err := json.Unmarshal(b, &overrides); err != nil {
			return err
		}
		languages = append(languages, overrides...)
	}
	abs, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	report, err := logs.Analyze(f.Args(), filepath.Dir(abs), logs.Options{All: *all, Rules: r, Languages: languages})
	if err != nil {
		return err
	}
	if err = logs.SaveReport(report, abs); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d events, %d categories, %.2f MiB, %.3f s\nreport: %s\n", report.Events, len(report.Groups), float64(report.Bytes)/1048576, float64(report.ElapsedMS)/1000, abs)
	return json.NewEncoder(os.Stdout).Encode(report)
}
func query(command string, args []string) error {
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	path := f.String("report", "report.json", "analysis report")
	out := f.String("out", "", "export output path")
	detail := f.Bool("details", false, "include occurrence details")
	before := f.Int("before", 20, "context lines before keyword or event")
	after := f.Int("after", 100, "context lines after keyword or event")
	offset := f.Int("offset", 0, "skip matching occurrences")
	limit := f.Int("limit", 20, "page size (max 100)")
	q := logs.Filter{Source: -1}
	f.StringVar(&q.Group, "group", "", "group id")
	f.StringVar(&q.Level, "level", "", "level")
	f.StringVar(&q.Keyword, "keyword", "", "case-insensitive keyword")
	f.StringVar(&q.Regex, "regex", "", "Go regular expression, per physical line")
	f.StringVar(&q.From, "from", "", "inclusive start time")
	f.StringVar(&q.To, "to", "", "inclusive end time")
	f.IntVar(&q.Source, "source", -1, "source index")
	if err := f.Parse(args); err != nil {
		return err
	}
	r, err := logs.LoadReport(*path)
	if err != nil {
		return err
	}
	if command == "export" {
		if *out == "" {
			return logs.Export(r, q, os.Stdout)
		}
		abs, err := filepath.Abs(*out)
		if err != nil {
			return err
		}
		if err := logs.SafeOutput(r, abs); err != nil {
			return err
		}
		reportPath, _ := filepath.Abs(*path)
		dest, _ := os.Stat(abs)
		reportStat, _ := os.Stat(reportPath)
		if abs == reportPath || dest != nil && reportStat != nil && os.SameFile(dest, reportStat) {
			return errors.New("export cannot overwrite the report")
		}
		for _, src := range r.Sources {
			if strings.EqualFold(abs, src.Path) {
				return errors.New("export cannot overwrite a source log")
			}
		}
		if strings.EqualFold(abs, r.IndexPath) || strings.EqualFold(abs, *path) {
			return errors.New("export cannot overwrite a report or index")
		}
		tmp, err := os.CreateTemp(filepath.Dir(abs), ".export-*")
		if err != nil {
			return err
		}
		defer os.Remove(tmp.Name())
		if err = logs.Export(r, q, tmp); err != nil {
			tmp.Close()
			return err
		}
		if err = tmp.Close(); err != nil {
			return err
		}
		return os.Rename(tmp.Name(), abs)
	}
	if *detail {
		d, more, err := logs.Details(r, q, *offset, *limit, logs.ContextOptions{Before: *before, After: *after})
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"details": d, "more": more})
	}
	g, err := logs.Summarize(r, q)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"groups": g, "sources": r.Sources})
}
func gui(args []string) error {
	defaultRules, err := logs.DefaultRuleDirectory()
	if err != nil {
		return err
	}
	f := flag.NewFlagSet("gui", flag.ContinueOnError)
	host := f.String("host", "0.0.0.0", "listen address; default enables all IPv4 and available IPv6 interfaces")
	port := f.Int("port", 8765, "listen port; 0 chooses a free port")
	allowedHosts := f.String("allowed-hosts", "", "comma-separated additional HTTP hostnames for DNS or reverse proxy access")
	cacheDir := f.String("cache-dir", "", "parent directory for temporary session files")
	noBrowser := f.Bool("no-browser", false, "do not open browser")
	keep := f.Bool("keep-cache", false, "keep session index after exit")
	ruleDir := f.String("rule-dir", defaultRules, "persistent per-language rule directory")
	if err := f.Parse(args); err != nil {
		return err
	}
	if *port < 0 || *port > 65535 {
		return errors.New("port must be between 0 and 65535")
	}
	dir, err := os.MkdirTemp(*cacheDir, "log-reviewer-")
	if err != nil {
		return err
	}
	if !*keep {
		defer os.RemoveAll(dir)
	}
	s := server.New(dir)
	if err := s.UseRuleDirectory(*ruleDir); err != nil {
		return err
	}
	defer s.Close()
	var hosts []string
	for _, value := range strings.Split(*allowedHosts, ",") {
		if value = strings.TrimSpace(value); value != "" {
			hosts = append(hosts, value)
		}
	}
	l, srv, err := server.ServeHost(s, *host, *port, hosts)
	if err != nil && *port != 0 {
		fmt.Fprintf(os.Stderr, "Cannot listen on %s:%d (%v); trying an automatically assigned port.\n", *host, *port, err)
		l, srv, err = server.ServeHost(s, *host, 0, hosts)
	}
	if err != nil {
		return err
	}
	listenHost, listenPort, _ := net.SplitHostPort(l.Addr().String())
	defer l.Close()
	var ipv6 net.Listener
	if *host == "0.0.0.0" {
		ipv6, err = net.Listen("tcp6", net.JoinHostPort("::", listenPort))
		if err != nil {
			fmt.Fprintf(os.Stderr, "IPv6 unavailable on port %s: %v\n", listenPort, err)
		} else {
			defer ipv6.Close()
		}
	}
	browserHost := listenHost
	if ip := net.ParseIP(listenHost); ip != nil && ip.IsUnspecified() {
		browserHost = "127.0.0.1"
		if *host == "::" {
			browserHost = "::1"
		}
	}
	url := accessURL(browserHost, listenPort, s.Token())
	fmt.Printf("Log Reviewer %s\nGUI: %s\nAPI token: %s\nCache: %s\nPress Ctrl+C to exit.\n", version, url, s.Token(), dir)
	fmt.Printf("Listen: %s\n", l.Addr())
	printAccessAddresses(listenHost, listenPort, s.Token(), ipv6 != nil)
	if len(f.Args()) > 0 {
		if err = s.Start(f.Args(), false, nil); err != nil {
			return err
		}
	}
	if !*noBrowser {
		openBrowser(url)
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(ch)
	done := make(chan error, 2)
	go func() { done <- srv.Serve(l) }()
	if ipv6 != nil {
		go func() { done <- srv.Serve(ipv6) }()
	}
	select {
	case err := <-done:
		return err
	case <-ch:
		l.Close()
		return nil
	}
}

func accessURL(host, port, token string) string {
	return "http://" + strings.ReplaceAll(net.JoinHostPort(host, port), "%", "%25") + "/#" + token
}

func printAccessAddresses(host, port, token string, extraIPv6 bool) {
	bound := net.ParseIP(host)
	if bound == nil || !bound.IsUnspecified() {
		fmt.Printf("Access: %s\n", accessURL(host, port, token))
		return
	}
	if bound.To4() != nil {
		fmt.Printf("Internal IPv4: %s\n", accessURL("127.0.0.1", port, token))
	}
	if extraIPv6 || bound.To4() == nil {
		fmt.Printf("Internal IPv6: %s\n", accessURL("::1", port, token))
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot enumerate network interfaces: %v\n", err)
		return
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			if ip.To4() != nil && bound.To4() != nil {
				fmt.Printf("External IPv4 (%s): %s\n", iface.Name, accessURL(ip.String(), port, token))
			} else if ip.To4() == nil && (extraIPv6 || bound.To4() == nil) {
				host := ip.String()
				if ip.IsLinkLocalUnicast() {
					host += "%" + strconv.Itoa(iface.Index)
				}
				fmt.Printf("External IPv6 (%s): %s\n", iface.Name, accessURL(host, port, token))
			}
		}
	}
	fmt.Println("External addresses are local interfaces; public NAT addresses and firewall reachability cannot be inferred.")
}
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}
