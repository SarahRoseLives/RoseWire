// tui/root_model.go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// sessionState represents the current view of the application.
type sessionState uint

const (
	loginView sessionState = iota
	appView   // The main application view after login
)

type RootModel struct {
	state         sessionState
	login         tea.Model
	app           tea.Model // Holds the main application model
	width, height int
	err           error
}

func NewRootModel() RootModel {
	return RootModel{
		state: loginView,
		login: NewLoginModel(),
	}
}

func (m RootModel) Init() tea.Cmd {
	return m.login.Init()
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	// This is sent by the login model when login is successful
	case loginSuccessMsg:
		m.state = appView

		// Get the server address from the login model's state to pass along
		loginModel := m.login.(LoginModel)
		serverAddr := loginModel.inputs[0].Value()

		m.app = NewAppModel(msg.Profile, serverAddr)

		// Pass the window size to the new model and initialize it
		cmds = append(cmds, m.app.Init(), func() tea.Msg {
			return tea.WindowSizeMsg{Width: m.width, Height: m.height}
		})

	case errMsg:
		m.err = msg
		return m, nil
	}

	// Route updates to the current view
	var currentView tea.Model
	switch m.state {
	case loginView:
		currentView = m.login
	case appView:
		currentView = m.app
	}

	// **FIX:** Make sure the view isn't nil before updating it.
	// This prevents the resize message from being sent to a non-existent app model.
	if currentView != nil {
		updatedView, viewCmd := currentView.Update(msg)
		cmds = append(cmds, viewCmd)

		switch m.state {
		case loginView:
			m.login = updatedView
		case appView:
			m.app = updatedView
		}
	}

	return m, tea.Batch(cmds...)
}

func (m RootModel) View() string {
	switch m.state {
	case loginView:
		return m.login.View()
	case appView:
		if m.app == nil {
			return "Initializing app..."
		}
		return m.app.View()
	default:
		return "Unknown state."
	}
}