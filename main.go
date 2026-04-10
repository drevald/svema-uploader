package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

//go:embed design/icon.svg
var iconData []byte

func main() {
	myApp := app.New()
	icon := fyne.NewStaticResource("icon.svg", iconData)
	myApp.SetIcon(icon)

	myWindow := myApp.NewWindow("Svema Photo Uploader")
	myWindow.SetIcon(icon)
	myWindow.Resize(fyne.NewSize(600, 600))
	myWindow.SetFixedSize(true)

	showLoginScreen(myWindow, myApp)

	myWindow.ShowAndRun()
}
