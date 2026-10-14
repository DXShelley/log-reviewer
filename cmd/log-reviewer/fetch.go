package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"log-reviewer/internal/logs"
	"log-reviewer/internal/remote"
)

func fetch(args []string) error {
	f := flag.NewFlagSet("fetch", flag.ContinueOnError)
	var options remote.Options
	f.StringVar(&options.Host, "host", "", "remote SSH hostname or IP")
	f.IntVar(&options.Port, "port", 22, "remote SSH port")
	f.StringVar(&options.User, "user", "", "remote account")
	f.StringVar(&options.Path, "path", "", "remote file or directory")
	f.StringVar(&options.Fingerprint, "fingerprint", "", "verified SHA256 host fingerprint")
	f.StringVar(&options.KnownHosts, "known-hosts", "", "OpenSSH known_hosts file")
	f.StringVar(&options.SSHDir, "ssh-dir", "", "local OpenSSH directory containing known_hosts")
	f.StringVar(&options.Pattern, "pattern", "*.log,*.log.*,*.txt", "comma-separated filename patterns for directories")
	f.BoolVar(&options.Recursive, "recursive", false, "include nested directories; symbolic links skipped")
	f.IntVar(&options.MaxFiles, "max-files", 1000, "maximum matching files (1-10000)")
	f.Int64Var(&options.MaxBytes, "max-bytes", 10<<30, "maximum total bytes (up to 10 GiB)")
	key := f.String("key", "", "private key file, read locally")
	passwordEnv := f.String("password-env", "LOG_REVIEWER_SFTP_PASSWORD", "environment variable containing password")
	passphraseEnv := f.String("passphrase-env", "LOG_REVIEWER_SFTP_PASSPHRASE", "environment variable containing private key passphrase")
	outputDir := f.String("out-dir", "remote-logs", "parent directory for downloaded session")
	analyzeFiles := f.Bool("analyze", false, "analyze downloaded files and emit report JSON")
	if err := f.Parse(args); err != nil {
		return err
	}
	options.Password = os.Getenv(*passwordEnv)
	options.Passphrase = os.Getenv(*passphraseEnv)
	if *key != "" {
		file, err := os.Open(*key)
		if err != nil {
			return err
		}
		bytes, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
		file.Close()
		if err != nil {
			return err
		}
		options.PrivateKey = string(bytes)
	}
	if options.Fingerprint == "" && options.KnownHosts == "" && options.SSHDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			options.KnownHosts = filepath.Join(home, ".ssh", "known_hosts")
		}
	}
	if err := options.Validate(); err != nil {
		return err
	}
	parent, err := filepath.Abs(*outputDir)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	destination, err := os.MkdirTemp(parent, "sftp-")
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	lastUpdate := time.Time{}
	manifest, err := remote.Download(ctx, options, destination, func(p remote.Progress) {
		if time.Since(lastUpdate) > 250*time.Millisecond || p.Stage == "complete" {
			fmt.Fprintf(os.Stderr, "%s: %d/%d files, %d/%d bytes\n", p.Stage, p.Files, p.TotalFiles, p.Bytes, p.Total)
			lastUpdate = time.Now()
		}
	})
	if err != nil {
		os.RemoveAll(destination)
		return err
	}
	fmt.Fprintf(os.Stderr, "Downloaded files and manifest: %s\n", destination)
	if !*analyzeFiles {
		return json.NewEncoder(os.Stdout).Encode(manifest)
	}
	paths := []string{}
	origins := map[string]logs.RemoteSource{}
	for _, file := range manifest.Files {
		paths = append(paths, file.LocalPath)
		origins[file.LocalPath] = logs.RemoteSource{Protocol: manifest.Protocol, Host: manifest.Host, Port: manifest.Port, Path: file.RemotePath, CapturedAt: manifest.CapturedAt, Changed: file.Changed, Modified: file.Modified}
	}
	ruleDir, err := logs.DefaultRuleDirectory()
	if err != nil {
		return err
	}
	languages, err := (&logs.RuleStore{Dir: ruleDir}).Overrides()
	if err != nil {
		return err
	}
	progress := &logs.Progress{}
	stopAnalysis := context.AfterFunc(ctx, func() { progress.Cancel.Store(true) })
	defer stopAnalysis()
	report, err := logs.Analyze(paths, destination, logs.Options{RemoteSources: origins, Languages: languages, Progress: progress})
	if err != nil {
		return err
	}
	if err = logs.SaveReport(report, filepath.Join(destination, "report.json")); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
