package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"rosewire/login"
	"rosewire/home"
)

type appState int

const (
	stateLogin appState = iota
	stateHome
)

type model struct {
	state     appState
	login     login.Model
	home      home.Model
	width     int
	height    int
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
			// On login complete, switch to home UI
			m.state = stateHome
			m.home = home.NewModel(m.login.Nickname, m.login.SelectedKey)
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