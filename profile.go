package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func isTerminal(f *os.File) bool {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), ioctlGetTermios, uintptr(unsafe.Pointer(&termios)))
	return errno == 0
}

// Bare `git config` on a terminal: pick a profile and set it on the current repository.
func configureProfile(git string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	u := ui{in: bufio.NewReader(os.Stdin), out: os.Stderr}
	if term := os.Getenv("TERM"); term != "" && term != "dumb" {
		u.menu = terminalMenu
	}
	p, save, err := pickProfile(u, cfg.Profiles, currentProfile(git))
	if err != nil {
		return err
	}
	if save {
		if err := saveProfile(p); err != nil {
			return err
		}
	}
	for _, kv := range [][2]string{{"user.name", p.Name}, {"user.email", p.Email}} {
		cmd := exec.Command(git, "config", "--local", kv[0], kv[1])
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				// git has already reported the failure.
				os.Exit(exit.ExitCode())
			}
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "better-git: this repository now commits as %s <%s>\n", p.Name, p.Email)
	return nil
}

// currentProfile is the identity git would commit with here; unset keys are left empty.
func currentProfile(git string) profile {
	get := func(key string) string {
		out, _ := exec.Command(git, "config", "--get", key).Output()
		return strings.TrimSpace(string(out))
	}
	return profile{Name: get("user.name"), Email: get("user.email")}
}

var errCancelled = errors.New("cancelled")

type ui struct {
	in  *bufio.Reader
	out io.Writer
	// menu picks one of items with the cursor keys; nil falls back to typing a number.
	menu func(items []string, start int) (int, error)
}

func (u ui) ask(prompt string) (string, error) {
	fmt.Fprint(u.out, prompt)
	line, err := u.in.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(u.out)
		return "", errCancelled
	}
	return strings.TrimSpace(line), nil
}

func (u ui) askRequired(prompt string) (string, error) {
	for {
		answer, err := u.ask(prompt)
		if err != nil || answer != "" {
			return answer, err
		}
		fmt.Fprintln(u.out, "A value is required.")
	}
}

func (u ui) chooseNumber(items []string, _ int) (int, error) {
	for _, item := range items {
		fmt.Fprintf(u.out, "  %s\n", item)
	}
	for {
		answer, err := u.ask("> ")
		if err != nil {
			return 0, err
		}
		if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(items) {
			return n - 1, nil
		}
		fmt.Fprintf(u.out, "Enter a number from 1 to %d.\n", len(items))
	}
}

// pickProfile returns the chosen profile and whether it is new and should be saved.
func pickProfile(u ui, profiles []profile, current profile) (profile, bool, error) {
	start, listed := 0, false
	items := make([]string, 0, len(profiles)+1)
	for i, p := range profiles {
		mark := ""
		if p == current {
			start, listed, mark = i, true, " (current)"
		}
		items = append(items, fmt.Sprintf("%d) %s <%s>%s", i+1, p.Name, p.Email, mark))
	}
	items = append(items, fmt.Sprintf("%d) Enter a name and email", len(profiles)+1))

	switch {
	case current == profile{}:
		fmt.Fprintln(u.out, "Current: not set")
	case !listed:
		fmt.Fprintf(u.out, "Current: %s <%s>\n", current.Name, current.Email)
	}

	if len(profiles) > 0 {
		fmt.Fprintln(u.out, "Select a profile for this repository:")
		choose := u.menu
		if choose == nil {
			choose = u.chooseNumber
		}
		n, err := choose(items, start)
		if err != nil {
			return profile{}, false, err
		}
		if n < len(profiles) {
			return profiles[n], false, nil
		}
	}

	var p profile
	var err error
	if p.Name, err = u.askRequired("user.name: "); err != nil {
		return profile{}, false, err
	}
	if p.Email, err = u.askRequired("user.email: "); err != nil {
		return profile{}, false, err
	}
	answer, err := u.ask("Save as a profile in ~/.bettergit? [y/N] ")
	if err != nil {
		return profile{}, false, err
	}
	answer = strings.ToLower(answer)
	return p, answer == "y" || answer == "yes", nil
}
