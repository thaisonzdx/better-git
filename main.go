package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Set at build time via -ldflags "-X main.version=..." from the git tag.
var version = "dev"

func main() {
	git, err := git()
	if err == nil {
		if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
			fmt.Println("bettergit version", version)
		}
		if len(os.Args) == 2 && os.Args[1] == "config" && isTerminal(os.Stdin) && isTerminal(os.Stderr) {
			if err = configureProfile(git); err == nil {
				return
			}
		} else {
			err = syscall.Exec(git, append([]string{"git"}, os.Args[1:]...), os.Environ())
		}
	}
	fmt.Fprintln(os.Stderr, "better-git:", err)
	os.Exit(1)
}

func git() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, err := os.Stat(exe)
	if err != nil {
		return "", err
	}
	isOriginal := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 && !os.SameFile(info, self)
	}

	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	if cfg.GitPath != "" && isOriginal(cfg.GitPath) {
		return cfg.GitPath, nil
	}

	// Config is missing or stale: fall back to the next git on PATH.
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		if path := filepath.Join(dir, "git"); isOriginal(path) {
			return path, nil
		}
	}
	return "", errors.New("original git not found; set gitPath in ~/.bettergit")
}
