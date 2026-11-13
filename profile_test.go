package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickProfile(t *testing.T) {
	saved := []profile{{"Ann", "ann@home.test"}, {"Ann Work", "ann@work.test"}}
	tests := []struct {
		name     string
		profiles []profile
		input    string
		want     profile
		save     bool
		wantErr  bool
	}{
		{"select saved", saved, "2\n", saved[1], false, false},
		{"direct input, saved", saved, "3\n Bob \nbob@x.test\ny\n", profile{"Bob", "bob@x.test"}, true, false},
		{"direct input, not saved", saved, "3\nBob\nbob@x.test\n\n", profile{"Bob", "bob@x.test"}, false, false},
		{"no profiles goes straight to input", nil, "Bob\nbob@x.test\nYES\n", profile{"Bob", "bob@x.test"}, true, false},
		{"invalid numbers are asked again", saved, "4\nann\n\n2\n", saved[1], false, false},
		{"empty name and email are asked again", nil, "\nBob\n\nbob@x.test\n\n", profile{"Bob", "bob@x.test"}, false, false},
		{"end of input at the menu", saved, "4\n", profile{}, false, true},
		{"end of input at the email", nil, "Bob\n\n", profile{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, save, err := pickProfile(lineUI(tt.input, io.Discard), tt.profiles, profile{})
			if (err != nil) != tt.wantErr || got != tt.want || save != tt.save {
				t.Errorf("got %+v, save=%v, err=%v", got, save, err)
			}
		})
	}
}

func lineUI(input string, out io.Writer) ui {
	return ui{in: bufio.NewReader(strings.NewReader(input)), out: out}
}

func TestPickProfileMenu(t *testing.T) {
	saved := []profile{{"Ann", "ann@home.test"}, {"Ann Work", "ann@work.test"}}
	u := lineUI("Bob\nbob@x.test\ny\n", io.Discard)
	var gotStart int
	u.menu = func(items []string, start int) (int, error) {
		gotStart = start
		return len(items) - 1, nil
	}
	got, save, err := pickProfile(u, saved, saved[1])
	if err != nil || !save || got != (profile{"Bob", "bob@x.test"}) || gotStart != 1 {
		t.Errorf("got %+v, save=%v, err=%v, start=%d", got, save, err, gotStart)
	}
}

func TestNextKey(t *testing.T) {
	tests := []struct {
		input string
		want  key
	}{
		{"\x1b[A", keyUp},
		{"\x1bOB", keyDown},
		{"k", keyUp},
		{"j", keyDown},
		{"\r", keyEnter},
		{"\x1b", keyCancel},
		{"q", keyCancel},
		{"\x03", keyCancel},
		{"3", key(3)},
		{"\x1b[1;5C", keyNone},
		{"x", keyNone},
		{"", keyNone},
	}
	for _, tt := range tests {
		input := tt.input
		read := func() (byte, bool) {
			if input == "" {
				return 0, false
			}
			b := input[0]
			input = input[1:]
			return b, true
		}
		if got := nextKey(read); got != tt.want || input != "" {
			t.Errorf("%q: got %d, want %d, unread %q", tt.input, got, tt.want, input)
		}
	}
}

func TestSelectMenu(t *testing.T) {
	items := []string{"1) a", "2) b", "3) c"}
	tests := []struct {
		name  string
		start int
		keys  []key
		want  int
	}{
		{"enter takes the starting row", 1, []key{keyEnter}, 1},
		{"down then enter", 0, []key{keyDown, keyDown, keyEnter}, 2},
		{"wraps both ways", 0, []key{keyUp, keyDown, keyUp, keyEnter}, 2},
		{"digit selects", 0, []key{key(2)}, 1},
		{"digit past the end is ignored", 0, []key{key(7), keyEnter}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := tt.keys
			next := func() key {
				if len(keys) == 0 {
					return keyCancel
				}
				k := keys[0]
				keys = keys[1:]
				return k
			}
			var out strings.Builder
			got, err := selectMenu(next, &out, items, tt.start, false)
			if err != nil || got != tt.want {
				t.Errorf("got %d, err=%v", got, err)
			}
			if strings.Contains(out.String(), "\x1b[1;36m") {
				t.Errorf("color written with color off: %q", out.String())
			}
		})
	}

	_, err := selectMenu(func() key { return keyCancel }, io.Discard, items, 0, true)
	if !errors.Is(err, errCancelled) {
		t.Errorf("cancel: err=%v", err)
	}
}

func TestPickProfileShowsCurrent(t *testing.T) {
	saved := []profile{{"Ann", "ann@home.test"}, {"Ann Work", "ann@work.test"}}
	tests := []struct {
		name    string
		current profile
		want    string
		marked  bool
	}{
		{"saved profile is marked", saved[1], "2) Ann Work <ann@work.test> (current)\n", true},
		{"unsaved identity is shown", profile{"Bob", "bob@x.test"}, "Current: Bob <bob@x.test>\n", false},
		{"nothing configured", profile{}, "Current: not set\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			pickProfile(lineUI("1\n", &out), saved, tt.current)
			if !strings.Contains(out.String(), tt.want) || strings.Contains(out.String(), "(current)") != tt.marked {
				t.Errorf("output:\n%s", out.String())
			}
		})
	}
}

func TestSaveProfileKeepsOtherKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".bettergit")
	if err := os.WriteFile(path, []byte(`{"gitPath": "/usr/bin/git", "future": [1]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, p := range []profile{{"Ann", "ann@home.test"}, {"Bob", "bob@x.test"}} {
		if err := saveProfile(p); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitPath != "/usr/bin/git" || len(cfg.Profiles) != 2 || cfg.Profiles[1] != (profile{"Bob", "bob@x.test"}) {
		t.Errorf("config after save: %+v", cfg)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"future"`) {
		t.Errorf("unknown key dropped: %s", data)
	}
}

func TestSaveProfileCreatesConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveProfile(profile{"Ann", "ann@home.test"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig()
	if err != nil || len(cfg.Profiles) != 1 {
		t.Errorf("cfg=%+v err=%v", cfg, err)
	}
}
