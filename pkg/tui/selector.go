package tui

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Item represents a selectable item, optionally under a group header.
type Item struct {
	Label  string
	Group  string // display header; items with the same Group are shown together
}

// MultiSelect presents an interactive arrow-key multi-select list.
// Returns the indices (into items) of the selected entries, or an error.
// The user navigates with ↑/↓, marks with Space, confirms with Enter,
// and cancels with Escape or q.
func MultiSelect(prompt string, items []Item) ([]int, error) {
	if len(items) == 0 {
		return nil, nil
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("could not set raw terminal: %w", err)
	}
	defer term.Restore(fd, oldState)

	selected := make([]bool, len(items))
	cursor := 0

	// Move cursor to next/prev real item (skip group headers in navigation).
	// We render headers inline so they don't occupy an items[] slot.

	render := func() {
		// Clear lines drawn previously — we track how many lines we printed.
		fmt.Print("\033[H\033[J") // clear screen
		fmt.Printf("  %s\n", prompt)
		fmt.Println("  ↑/↓ move  Space select  Enter confirm  Esc cancel")
		fmt.Println()

		lastGroup := ""
		for i, item := range items {
			if item.Group != lastGroup {
				if lastGroup != "" {
					fmt.Println()
				}
				fmt.Printf("  \033[1;4m%s\033[0m\n", item.Group)
				lastGroup = item.Group
			}

			mark := " "
			if selected[i] {
				mark = "✓"
			}

			line := fmt.Sprintf("  [%s] %s", mark, item.Label)
			if i == cursor {
				fmt.Printf("\033[7m%s\033[0m\n", line) // reverse video for cursor
			} else {
				fmt.Println(line)
			}
		}

		// Count selected
		n := 0
		for _, s := range selected {
			if s {
				n++
			}
		}
		fmt.Printf("\n  %d selected\n", n)
	}

	render()

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return nil, err
		}

		switch {
		case n == 1 && buf[0] == ' ':
			selected[cursor] = !selected[cursor]

		case n == 1 && (buf[0] == 13 || buf[0] == 10): // Enter
			var out []int
			for i, s := range selected {
				if s {
					out = append(out, i)
				}
			}
			fmt.Print("\033[H\033[J")
			return out, nil

		case n == 1 && (buf[0] == 27 || buf[0] == 'q'): // Esc or q
			fmt.Print("\033[H\033[J")
			return nil, nil

		case n == 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'A': // up arrow
			if cursor > 0 {
				cursor--
			}

		case n == 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'B': // down arrow
			if cursor < len(items)-1 {
				cursor++
			}

		case n == 1 && strings.ContainsRune("kK", rune(buf[0])): // vim-style up
			if cursor > 0 {
				cursor--
			}

		case n == 1 && strings.ContainsRune("jJ", rune(buf[0])): // vim-style down
			if cursor < len(items)-1 {
				cursor++
			}
		}

		render()
	}
}
