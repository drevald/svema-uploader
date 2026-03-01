//go:build !prod

package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showLoginScreen(w fyne.Window, a fyne.App) {
	header := widget.NewLabelWithStyle("Welcome to Svema Uploader", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	envRadio := widget.NewRadioGroup([]string{"Public", "Local", "Test"}, func(selected string) {
		switch selected {
		case "Public":
			SetBaseUrl(BaseUrlPublic)
		case "Local":
			SetBaseUrl(BaseUrlLocal)
		case "Test":
			SetBaseUrl(BaseUrlTest)
		}
	})
	envRadio.Horizontal = true
	envRadio.Selected = "Public"
	SetBaseUrl(BaseUrlPublic)

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Enter Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter Password")

	loginButton := widget.NewButton("Login", func() {
		if usernameEntry.Text == "" {
			showError(fmt.Errorf("Please enter a username"), w)
			return
		}
		if passwordEntry.Text == "" {
			showError(fmt.Errorf("Please enter a password"), w)
			return
		}

		token, userId, err := LoginUser(usernameEntry.Text, passwordEntry.Text)
		if err != nil {
			showError(fmt.Errorf("Login failed: %v", err), w)
			return
		}

		SetAuthToken(token)

		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "/"
		}
		showFileBrowser(w, a, userId, homeDir)
	})

	content := container.NewVBox(
		widget.NewLabel(""),
		header,
		widget.NewLabel(""),
		widget.NewLabel("Environment:"),
		envRadio,
		widget.NewLabel(""),
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		widget.NewLabel(""),
		loginButton,
	)

	w.SetContent(container.NewCenter(content))
}
