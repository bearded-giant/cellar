package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/helpers"
	"github.com/bearded-giant/cellar/internal/history"
	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/internal/tui/ui"
	"github.com/bearded-giant/cellar/models"
)

var version = "dev"

func roSuffix(ro bool) string {
	if ro {
		return " [read-only]"
	}
	return ""
}

func main() {
	fs := flag.NewFlagSet("cellar", flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "Print version and exit")
	configPath := fs.String("config", "", "Path to config.toml (defaults to XDG config dir)")
	addConn := fs.Bool("add-connection", false, "Add/replace a connection in the config and exit (non-interactive)")
	connName := fs.String("name", "", "Connection name (with --add-connection)")
	connURL := fs.String("url", "", "Connection URL (with --add-connection)")
	connProvider := fs.String("provider", "", "Provider override; inferred from the URL when empty")
	connSchema := fs.String("schema", "", "Default schema to auto-expand on connect (postgres, with --add-connection)")
	connRO := fs.Bool("read-only", false, "Mark the connection read-only (with --add-connection)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: cellar [flags]\n\n")
		fmt.Fprintf(os.Stderr, "A Bubble Tea database client.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "      --version              Print version and exit\n")
		fmt.Fprintf(os.Stderr, "      --config string        Path to config.toml\n")
		fmt.Fprintf(os.Stderr, "      --add-connection       Add/replace a connection, then exit\n")
		fmt.Fprintf(os.Stderr, "      --name string          Connection name (with --add-connection)\n")
		fmt.Fprintf(os.Stderr, "      --url string           Connection URL (with --add-connection)\n")
		fmt.Fprintf(os.Stderr, "      --provider string      Provider override (inferred from URL if empty)\n")
		fmt.Fprintf(os.Stderr, "      --schema string        Default schema to auto-expand (postgres)\n")
		fmt.Fprintf(os.Stderr, "      --read-only            Mark the connection read-only\n")
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if *showVersion {
		fmt.Printf("cellar %s\n", version)
		return
	}

	path := *configPath
	if path == "" {
		p, err := config.DefaultConfigFile()
		if err != nil {
			log.Fatalf("resolve config path: %v", err)
		}
		path = p
	}

	if *addConn {
		if *connName == "" || *connURL == "" {
			log.Fatal("--add-connection requires --name and --url")
		}
		provider := *connProvider
		if provider == "" {
			if parsed, perr := helpers.ParseConnectionString(*connURL); perr == nil {
				provider = parsed.Driver
			}
		}
		conn := models.Connection{Name: *connName, URL: *connURL, Provider: provider, DefaultSchema: *connSchema, ReadOnly: *connRO}
		if err := config.UpsertConnection(path, conn); err != nil {
			log.Fatalf("add connection: %v", err)
		}
		fmt.Printf("cellar: saved connection %q (%s)%s\n", *connName, provider, roSuffix(*connRO))
		return
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.AppConfig != nil && cfg.AppConfig.MaxQueryHistoryPerConnection > 0 {
		history.MaxPerConnection = cfg.AppConfig.MaxQueryHistoryPerConnection
	}

	cmds := commands.New(cfg)
	m := ui.New(cmds)
	m.Version = version

	sendFunc := func(tea.Msg) {}
	m.SendFunc = &sendFunc

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	*m.SendFunc = p.Send
	finalModel, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}
	if fm, ok := finalModel.(ui.Model); ok {
		fm.PersistQueryState() // quit backstop for the in-flight scratch SQL
		if fm.ActiveTunnel != nil {
			_ = fm.ActiveTunnel.Close()
		}
	}
}
