package main

import (
	"fmt"
	"os"

	"github.com/bearded-giant/cellar/internal/backup"
	"github.com/bearded-giant/cellar/internal/tui/config"
)

// configuredBackupDir reads the BackupDir setting; "" when unset/unreadable
// (export then falls back to the current directory).
func configuredBackupDir() string {
	path, err := config.DefaultConfigFile()
	if err != nil {
		return ""
	}
	_, v, err := config.GetAppSetting(path, "BackupDir")
	if err != nil {
		return ""
	}
	return backup.ExpandHome(v)
}

func runConfigCommand(args []string) {
	path, err := config.DefaultConfigFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
		os.Exit(1)
	}
	usage := func() {
		fmt.Fprintln(os.Stderr, "usage: cellar config list | get <key> | set <key> <value>")
		os.Exit(2)
	}
	if len(args) < 2 {
		usage()
	}
	switch args[1] {
	case "list":
		rows, err := config.ListAppSettings(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
			os.Exit(1)
		}
		for _, r := range rows {
			fmt.Printf("%-32s %s\n", r[0], r[1])
		}
	case "get":
		if len(args) < 3 {
			usage()
		}
		name, v, err := config.GetAppSetting(path, args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s = %s\n", name, v)
	case "set":
		if len(args) < 4 {
			usage()
		}
		name, err := config.SetAppSetting(path, args[2], args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cellar: %s = %s saved to %s\n", name, args[3], path)
	default:
		usage()
	}
}

func runBackupCommand(args []string) {
	switch args[0] {
	case "export":
		out := ""
		if len(args) > 1 {
			out = args[1]
		}
		path, err := backup.Export(out, configuredBackupDir())
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar export: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cellar: backed up connections, saved queries, buffers and history to %s\n", path)
		fmt.Println("cellar: the archive contains connection credentials — store it somewhere safe")
	case "import":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: cellar import <backup.tar.gz>")
			os.Exit(2)
		}
		aside, err := backup.Import(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "cellar import: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("cellar: restored from %s\n", args[1])
		if aside != "" {
			fmt.Printf("cellar: previous config kept at %s (delete it once you're happy)\n", aside)
		}
	}
}
