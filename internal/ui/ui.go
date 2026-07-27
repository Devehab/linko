// Package ui contains the small terminal helpers linko uses for output and
// interactive prompts. No third-party dependencies.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
)

var useColor = detectColor()

func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// SetColor forces colour output on or off (used by tests and --no-color).
func SetColor(on bool) { useColor = on }

func paint(code, s string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// Colour helpers.
func Bold(s string) string   { return paint("1", s) }
func Dim(s string) string    { return paint("2", s) }
func Red(s string) string    { return paint("31", s) }
func Green(s string) string  { return paint("32", s) }
func Yellow(s string) string { return paint("33", s) }
func Cyan(s string) string   { return paint("36", s) }

// Status lines.
func Success(format string, a ...any) {
	fmt.Printf("%s %s\n", Green("✓"), fmt.Sprintf(format, a...))
}

func Fail(format string, a ...any) {
	fmt.Printf("%s %s\n", Red("✗"), fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Printf("%s %s\n", Yellow("!"), fmt.Sprintf(format, a...))
}

func Info(format string, a ...any) {
	fmt.Printf("%s %s\n", Dim("·"), fmt.Sprintf(format, a...))
}

func Line(format string, a ...any) { fmt.Printf(format+"\n", a...) }

func Blank() { fmt.Println() }

func Header(s string) {
	fmt.Println()
	fmt.Println(Bold(s))
}

// Table renders aligned columns.
func Table(w io.Writer, header []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	if len(header) > 0 {
		fmt.Fprintln(tw, Dim(strings.Join(header, "\t")))
	}
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
}

// Prompter wraps stdin for the interactive wizards.
type Prompter struct {
	r *bufio.Reader
}

// NewPrompter reads from os.Stdin.
func NewPrompter() *Prompter { return &Prompter{r: bufio.NewReader(os.Stdin)} }

// NewPrompterFrom reads from an arbitrary reader (tests).
func NewPrompterFrom(r io.Reader) *Prompter { return &Prompter{r: bufio.NewReader(r)} }

// Ask prompts for a line of text, returning def when the user just hits enter.
func (p *Prompter) Ask(label, def string) (string, error) {
	if def != "" {
		fmt.Printf("%s %s ", label, Dim("["+def+"]"))
	} else {
		fmt.Printf("%s ", label)
	}
	line, err := p.r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

// AskRequired keeps asking until a non-empty value passes validate.
func (p *Prompter) AskRequired(label, def string, validate func(string) error) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		v, err := p.Ask(label, def)
		if err != nil {
			return "", err
		}
		if v == "" {
			Warn("a value is required")
			continue
		}
		if validate != nil {
			if verr := validate(v); verr != nil {
				Warn("%v", verr)
				continue
			}
		}
		return v, nil
	}
	return "", fmt.Errorf("too many invalid answers")
}

// AskSecret prompts without echoing the input where the platform allows it.
func (p *Prompter) AskSecret(label string) (string, error) {
	fmt.Printf("%s ", label)
	restore := disableEcho()
	line, err := p.r.ReadString('\n')
	restore()
	fmt.Println()
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func disableEcho() func() {
	if runtime.GOOS == "windows" {
		return func() {}
	}
	stty, err := exec.LookPath("stty")
	if err != nil {
		return func() {}
	}
	off := exec.Command(stty, "-echo")
	off.Stdin = os.Stdin
	if err := off.Run(); err != nil {
		return func() {}
	}
	return func() {
		on := exec.Command(stty, "echo")
		on.Stdin = os.Stdin
		_ = on.Run()
	}
}

// Confirm asks a yes/no question.
func (p *Prompter) Confirm(label string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	fmt.Printf("%s %s ", label, Dim("["+hint+"]"))
	line, err := p.r.ReadString('\n')
	if err != nil && line == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "":
		return def
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return def
	}
}

// Choose presents a numbered menu and returns the zero-based selection.
func (p *Prompter) Choose(label string, options []string) (int, error) {
	fmt.Println(label)
	for i, o := range options {
		fmt.Printf("  %d. %s\n", i+1, o)
	}
	for attempt := 0; attempt < 5; attempt++ {
		fmt.Printf("%s ", "Choose:")
		line, err := p.r.ReadString('\n')
		if err != nil && line == "" {
			return -1, err
		}
		n, cerr := strconv.Atoi(strings.TrimSpace(line))
		if cerr == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		Warn("enter a number between 1 and %d", len(options))
	}
	return -1, fmt.Errorf("no valid choice given")
}
