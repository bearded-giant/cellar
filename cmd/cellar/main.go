package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bearded-giant/cellar/internal/history"
	"github.com/bearded-giant/cellar/internal/tui/commands"
	"github.com/bearded-giant/cellar/internal/tui/config"
	"github.com/bearded-giant/cellar/internal/tui/ui"
)

var version = "dev"

func main() {
	fs := flag.NewFlagSet("cellar", flag.ContinueOnError)
	showVersion := fs.Bool("version", false, "Print version and exit")
	configPath := fs.String("config", "", "Path to config.toml (defaults to XDG config dir)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: cellar [flags]\n\n")
		fmt.Fprintf(os.Stderr, "A Bubble Tea connection manager for cellar.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "      --version        Print version and exit\n")
		fmt.Fprintf(os.Stderr, "      --config string  Path to config.toml\n")
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
	if fm, ok := finalModel.(ui.Model); ok && fm.ActiveTunnel != nil {
		_ = fm.ActiveTunnel.Close()
	}
}
