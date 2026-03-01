package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Svema Photo Uploader")
	myWindow.Resize(fyne.NewSize(800, 600))

	showLoginScreen(myWindow, myApp)

	myWindow.ShowAndRun()
}
