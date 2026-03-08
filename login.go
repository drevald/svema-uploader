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
	header := widget.NewLabelWithStyle("Добро пожаловать в Svema Uploader", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

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
	usernameEntry.SetPlaceHolder("Введите имя пользователя")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Введите пароль")

	forgotButton := widget.NewButton("Забыли пароль?", func() {
		showForgotPasswordScreen(w, a)
	})

	loginButton := widget.NewButton("Войти", func() {
		if usernameEntry.Text == "" {
			showError(fmt.Errorf("Пожалуйста, введите имя пользователя"), w)
			return
		}
		if passwordEntry.Text == "" {
			showError(fmt.Errorf("Пожалуйста, введите пароль"), w)
			return
		}

		token, userId, err := LoginUser(usernameEntry.Text, passwordEntry.Text)
		if err != nil {
			showError(fmt.Errorf("Ошибка входа: %v", err), w)
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
		widget.NewLabel("Окружение:"),
		envRadio,
		widget.NewLabel(""),
		widget.NewLabel("Имя пользователя:"),
		usernameEntry,
		widget.NewLabel("Пароль:"),
		passwordEntry,
		widget.NewLabel(""),
		loginButton,
		forgotButton,
	)

	w.SetContent(container.NewCenter(content))
}
