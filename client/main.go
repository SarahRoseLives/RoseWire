package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"rosewire/home"
	"rosewire/login"
)

type appState int

const (
	stateLogin appState = iota
	stateHome
)

type model struct {
	state  appState
	login  login.Model
	home   home.Model
	width  int
	height int
}

// Initialize app with login screen
func initialModel() model {
	lm := login.NewModel()
	return model{
		state: stateLogin,
		login: lm,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.login.Init(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateLogin:
		lm, cmd := m.login.Update(msg)
		m.login = lm
		if m.login.Done {
			// On login complete, create dirs and connect to services
			if err := home.EnsureUserDirs(); err != nil {
				// A real app should display this error gracefully
				fmt.Printf("Fatal: Could not create user directories: %v\n", err)
				return m, tea.Quit
			}

			// Create and connect chat client
			chatClient := home.NewChatClient(m.login.Nickname, m.login.SelectedKey, "127.0.0.1:2222")
			go func() {
				err := chatClient.Connect()
				if err != nil {
					// Log error; a more robust solution would use a tea.Msg to show in UI
					log.Printf("Chat connection failed: %v", err)
				}
			}()

			// Switch to home UI, passing the connected client
			m.state = stateHome
			m.home = home.NewModel(m.login.Nickname, m.login.SelectedKey, chatClient)
			return m, m.home.Init()
		}
		return m, cmd
	case stateHome:
		hm, cmd := m.home.Update(msg)
		m.home = hm
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateLogin:
		return m.login.View()
	case stateHome:
		return m.home.View()
	}
	return ""
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}