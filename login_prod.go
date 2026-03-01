//go:build prod

package main

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func showLoginScreen(w fyne.Window, a fyne.App) {
	SetBaseUrl(BaseUrlPublic)
	header := widget.NewLabelWithStyle("Welcome to Svema Uploader", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Enter Username")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter Password")

	forgotButton := widget.NewButton("Forgot password?", func() {
		showForgotPasswordScreen(w, a)
	})

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
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		widget.NewLabel(""),
		loginButton,
		forgotButton,
	)

	w.SetContent(container.NewCenter(content))
}
