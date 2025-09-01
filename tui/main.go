// main.go
package main

import (
	"fmt"
	"os"
	"rosetui/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// If you want to log to a file, you can uncomment the following lines.
	// f, err := tea.LogToFile("debug.log", "debug")
	// if err != nil {
	// 	fmt.Println("fatal:", err)
	// 	os.Exit(1)
	// }
	// defer f.Close()

	p := tea.NewProgram(tui.NewRootModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}