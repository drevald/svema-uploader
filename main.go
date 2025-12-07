package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/rwcarlsen/goexif/exif"
)

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Svema Photo Uploader")
	myWindow.Resize(fyne.NewSize(800, 600))

	showLoginScreen(myWindow, myApp)

	myWindow.ShowAndRun()
}

// Helper functions for recent directories
func getRecentDirs() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []string{}
	}

	prefsFile := filepath.Join(homeDir, ".svema-uploader-recent")
	data, err := os.ReadFile(prefsFile)
	if err != nil {
		return []string{}
	}

	var recent []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			recent = append(recent, line)
		}
	}
	return recent
}

func saveRecentDir(path string) {
	recent := getRecentDirs()

	// Remove if already exists
	newRecent := []string{path}
	for _, p := range recent {
		if p != path && len(newRecent) < 5 {
			newRecent = append(newRecent, p)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}

	prefsFile := filepath.Join(homeDir, ".svema-uploader-recent")
	content := strings.Join(newRecent, "\n")
	os.WriteFile(prefsFile, []byte(content), 0644)
}

func showLoginScreen(w fyne.Window, a fyne.App) {
	// Header
	header := widget.NewLabelWithStyle("Welcome to Svema Uploader", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Environment Selection
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
	envRadio.Selected = "Local" // Default to Local

	// Username Input
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Enter Username")

	// Password Input
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter Password")

	// Login Button
	loginButton := widget.NewButton("Login", func() {
		if usernameEntry.Text == "" {
			showError(fmt.Errorf("Please enter a username"), w)
			return
		}
		if passwordEntry.Text == "" {
			showError(fmt.Errorf("Please enter a password"), w)
			return
		}

		// Call login API to get JWT token
		token, userId, err := LoginUser(usernameEntry.Text, passwordEntry.Text)
		if err != nil {
			showError(fmt.Errorf("Login failed: %v", err), w)
			return
		}

		// Store the JWT token globally for use in API calls
		SetAuthToken(token)

		// Navigate to File Browser (start at home dir or root)
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "/"
		}
		showFileBrowser(w, a, userId, homeDir)
	})

	// Layout
	content := container.NewVBox(
		widget.NewLabel(""), // Spacer
		header,
		widget.NewLabel(""), // Spacer
		widget.NewLabel("Environment:"),
		envRadio,
		widget.NewLabel(""), // Spacer
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		widget.NewLabel(""), // Spacer
		loginButton,
	)

	// Center the content
	w.SetContent(container.NewCenter(content))
}

// extractExifThumbnail attempts to extract the embedded EXIF thumbnail from a JPEG file
func extractExifThumbnail(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return nil, err
	}

	// Try to get the thumbnail
	thumbData, err := x.JpegThumbnail()
	if err != nil {
		return nil, err
	}

	return thumbData, nil
}

func showFileBrowser(w fyne.Window, a fyne.App, userId int, currentPath string) {
	// Header
	// Header

	pathLabel := widget.NewLabel("Current: " + currentPath)
	pathLabel.Wrapping = fyne.TextWrapBreak

	// Navigation buttons
	upButton := widget.NewButtonWithIcon("Up", theme.NavigateBackIcon(), func() {
		parent := filepath.Dir(currentPath)
		if parent != currentPath {
			showFileBrowser(w, a, userId, parent)
		}
	})

	homeButton := widget.NewButtonWithIcon("Home", theme.HomeIcon(), func() {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			showFileBrowser(w, a, userId, homeDir)
		}
	})

	drivesButton := widget.NewButton("Drives", func() {
		// Create a list of available drives (C: through Z:)
		var drives []string
		for drive := 'C'; drive <= 'Z'; drive++ {
			drivePath := string(drive) + ":\\"
			if _, err := os.Stat(drivePath); err == nil {
				drives = append(drives, drivePath)
			}
		}

		if len(drives) == 0 {
			dialog.ShowInformation("No Drives", "No drives found", w)
			return
		}

		// Create buttons for each drive
		var driveButtons []fyne.CanvasObject
		for _, drive := range drives {
			driveCopy := drive // Capture for closure
			btn := widget.NewButton(driveCopy, func() {
				showFileBrowser(w, a, userId, driveCopy)
			})
			driveButtons = append(driveButtons, btn)
		}

		driveList := container.NewVBox(driveButtons...)
		dialog.ShowCustom("Select Drive", "Cancel", driveList, w)
	})

	selectButton := widget.NewButton("Select This Directory", func() {
		saveRecentDir(currentPath)
		showUploadConfig(w, a, userId, currentPath)
	})
	selectButton.Importance = widget.HighImportance

	files, err := os.ReadDir(currentPath)
	if err != nil {
		showError(err, w)
		return
	}

	// Process folders first, then files - exclude folders starting with .
	var folders []os.DirEntry
	var filesList []os.DirEntry

	for _, f := range files {
		// Skip hidden folders (starting with .)
		if f.IsDir() && strings.HasPrefix(f.Name(), ".") {
			continue
		}

		if f.IsDir() {
			folders = append(folders, f)
		} else {
			filesList = append(filesList, f)
		}
	}

	gridContent := container.NewGridWithColumns(4) // Adjust columns as needed

	// Add folders to grid
	for _, f := range folders {
		name := f.Name()
		path := filepath.Join(currentPath, name)
		btn := widget.NewButtonWithIcon(name, theme.FolderIcon(), func() {
			showFileBrowser(w, a, userId, path)
		})
		// Truncate long names
		if len(name) > 15 {
			btn.SetText(name[:12] + "...")
		}
		gridContent.Add(btn)
	}

	// Add files to grid with placeholders first
	// Count images first
	imageCount := 0
	for _, f := range filesList {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tiff" || ext == ".tif" {
			imageCount++
		}
	}

	// Disable thumbnails to keep app responsive
	loadThumbnails := false // Disabled for performance

	type thumbnailJob struct {
		path      string
		imgWidget *canvas.Image
	}

	var jobs chan thumbnailJob
	if loadThumbnails {
		jobs = make(chan thumbnailJob, imageCount)
	}

	for _, f := range filesList {
		name := f.Name()
		path := filepath.Join(currentPath, name)
		ext := strings.ToLower(filepath.Ext(name))
		isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tiff" || ext == ".tif"

		// Create a container that will hold the image/icon
		imgHolder := container.NewStack()

		// Initial placeholder or generic icon
		icon := theme.FileIcon()
		if isImage {
			icon = theme.FileImageIcon()
		}

		// We use an image object that we can update later
		imgWidget := canvas.NewImageFromResource(icon)
		imgWidget.FillMode = canvas.ImageFillContain
		imgWidget.SetMinSize(fyne.NewSize(100, 100))

		imgHolder.Add(imgWidget)

		label := widget.NewLabel(name)
		label.Alignment = fyne.TextAlignCenter
		label.Wrapping = fyne.TextTruncate

		item := container.NewVBox(imgHolder, label)
		gridContent.Add(item)

		// Queue image loading job (only if enabled)
		if isImage && loadThumbnails {
			jobs <- thumbnailJob{path: path, imgWidget: imgWidget}
		}
	}

	if loadThumbnails && jobs != nil {
		close(jobs)

		// Start worker pool to load thumbnails in parallel
		numWorkers := 4
		for i := 0; i < numWorkers; i++ {
			go func() {
				for job := range jobs {
					var data []byte
					var err error

					// Try to extract EXIF thumbnail first (much smaller and faster)
					data, err = extractExifThumbnail(job.path)
					if err != nil {
						// Fallback to loading full image if EXIF thumbnail not available
						data, err = os.ReadFile(job.path)
						if err != nil {
							continue
						}
					}

					// Create static resource
					res := fyne.NewStaticResource(filepath.Base(job.path), data)

					// Update UI
					job.imgWidget.Resource = res
					job.imgWidget.Refresh()
				}
			}()
		}
	}

	// Scroll container for main content
	scroll := container.NewVScroll(gridContent)

	// Logout button
	logoutButton := widget.NewButton("Logout", func() {
		showLoginScreen(w, a)
	})

	// Top bar
	topBar := container.NewBorder(nil, nil, container.NewHBox(upButton, homeButton, drivesButton), logoutButton, pathLabel)

	// Recent directories section (simple, at the top)
	recentDirs := getRecentDirs()
	var mainContent fyne.CanvasObject

	if len(recentDirs) > 0 {
		recentLabel := widget.NewLabel("Recent: ")
		recentLabel.TextStyle = fyne.TextStyle{Bold: true}

		recentBtns := []fyne.CanvasObject{recentLabel}
		for i, dir := range recentDirs {
			if i >= 3 { // Limit to 3 for space
				break
			}
			// Check if directory still exists
			if _, err := os.Stat(dir); err == nil {
				dirPath := dir // Capture for closure
				// Show just the basename
				btnText := filepath.Base(dirPath)
				if len(btnText) > 20 {
					btnText = btnText[:17] + "..."
				}
				btn := widget.NewButton(btnText, func() {
					showFileBrowser(w, a, userId, dirPath)
				})
				btn.Importance = widget.LowImportance
				recentBtns = append(recentBtns, btn)
			}
		}

		recentBar := container.NewHBox(recentBtns...)
		mainContent = container.NewBorder(
			container.NewVBox(topBar, recentBar, widget.NewSeparator()),
			selectButton,
			nil,
			nil,
			scroll,
		)
	} else {
		mainContent = container.NewBorder(topBar, selectButton, nil, nil, scroll)
	}

	w.SetContent(mainContent)
}

func showUploadConfig(w fyne.Window, a fyne.App, userId int, selectedPath string) {
	// Header
	header := widget.NewLabelWithStyle("Upload Configuration", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	pathLabel := widget.NewLabel("Directory: " + selectedPath)
	pathLabel.Wrapping = fyne.TextWrapBreak

	// Options
	// Grouping Radio
	groupingRadio := widget.NewRadioGroup([]string{"Group by Date", "Group by Folder"}, nil)
	groupingRadio.Horizontal = true
	groupingRadio.Selected = "Group by Date" // Default

	// Recursive Checkbox
	recursiveCheck := widget.NewCheck("Process subdirectories recursively", nil)
	recursiveCheck.Checked = false

	// Logic to enable/disable recursive check
	groupingRadio.OnChanged = func(selected string) {
		if selected == "Group by Date" {
			recursiveCheck.Enable()
		} else {
			recursiveCheck.Disable()
			recursiveCheck.SetChecked(true) // Implied for Folder mode? Or irrelevant?
			// If Group by Folder, we usually want to process folders.
			// Let's assume Group by Folder implies processing subfolders (recursive=true).
		}
	}

	// Progress bar
	progressBar := widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1
	progressBar.Hide()

	// Status label
	statusLabel := widget.NewLabel("")

	// Log
	logData := []string{}
	logList := widget.NewList(
		func() int { return len(logData) },
		func() fyne.CanvasObject {
			// Create a container with background color
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapWord
			return container.NewStack(label)
		},
		func(i int, o fyne.CanvasObject) {
			stack := o.(*fyne.Container)

			// Parse the message format: Status|Filename|Album: name|Date: date|GPS: lat, lon (optional)|Error: details (optional)
			msg := logData[i]
			parts := strings.Split(msg, "|")

			var displayText string
			var bgColor color.Color

			if len(parts) >= 3 {
				status := parts[0]
				filename := parts[1]
				albumInfo := parts[2]
				dateInfo := ""
				gpsInfo := ""
				errorInfo := ""

				if len(parts) >= 4 {
					dateInfo = parts[3]
				}
				if len(parts) >= 5 {
					// Could be GPS or Error
					if strings.HasPrefix(parts[4], "GPS:") {
						gpsInfo = parts[4]
						if len(parts) >= 6 {
							errorInfo = parts[5]
						}
					} else if strings.HasPrefix(parts[4], "Error:") {
						errorInfo = parts[4]
					} else {
						// Fallback
						gpsInfo = parts[4]
					}
				}

				// Format the display text
				if status == "Failed" {
					bgColor = color.NRGBA{R: 255, G: 200, B: 200, A: 255} // Light pink
					displayText = fmt.Sprintf("%s - %s - %s", filename, albumInfo, dateInfo)
					if gpsInfo != "" {
						displayText += fmt.Sprintf(" - %s", gpsInfo)
					}
					if errorInfo != "" {
						displayText += fmt.Sprintf(" - %s", errorInfo)
					}
				} else {
					bgColor = color.NRGBA{R: 200, G: 255, B: 200, A: 255} // Light green
					displayText = fmt.Sprintf("%s - %s - %s", filename, albumInfo, dateInfo)
					if gpsInfo != "" {
						displayText += fmt.Sprintf(" - %s", gpsInfo)
					}
				}
			} else {
				// Fallback for old format
				displayText = msg
				if strings.HasPrefix(msg, "Failed") {
					bgColor = color.NRGBA{R: 255, G: 200, B: 200, A: 255}
				} else {
					bgColor = color.NRGBA{R: 200, G: 255, B: 200, A: 255}
				}
			}

			// Create background rectangle
			bg := canvas.NewRectangle(bgColor)

			// Create label with black text
			label := widget.NewLabel(displayText)
			label.Wrapping = fyne.TextWrapWord
			label.TextStyle = fyne.TextStyle{}

			// Update stack with background and label
			stack.Objects = []fyne.CanvasObject{bg, container.NewPadded(label)}
			stack.Refresh()
		},
	)
	logContainer := container.NewVScroll(logList)
	logContainer.SetMinSize(fyne.NewSize(0, 150))

	// Buttons
	backButton := widget.NewButton("Back", func() {
		showFileBrowser(w, a, userId, selectedPath)
	})

	var uploadButton *widget.Button
	var cancelButton *widget.Button
	var pauseButton *widget.Button
	var control *UploadControl

	cancelButton = widget.NewButton("Cancel", func() {
		if control != nil {
			control.Cancel()
			statusLabel.SetText("Cancelling...")
			cancelButton.Disable() // Prevent double click
			pauseButton.Disable()
		}
	})
	cancelButton.Disable()
	cancelButton.Importance = widget.DangerImportance

	pauseButton = widget.NewButton("Pause", nil)
	pauseButton.OnTapped = func() {
		if control != nil {
			if control.IsPaused() {
				control.SetPaused(false)
				pauseButton.SetText("Pause")
				statusLabel.SetText("Resuming...")
			} else {
				control.SetPaused(true)
				pauseButton.SetText("Resume")
				statusLabel.SetText("Paused")
			}
		}
	}
	pauseButton.Disable()

	uploadButton = widget.NewButton("Start Upload", func() {
		uploadButton.Disable()
		backButton.Disable()
		cancelButton.Enable()
		pauseButton.Enable()
		pauseButton.SetText("Pause")

		progressBar.Show()
		statusLabel.SetText("Starting upload...")
		logData = []string{}
		logList.Refresh()

		control = &UploadControl{}

		go func() {
			progressCallback := func(current, total int, message string) {
				progressBar.Max = float64(total)
				progressBar.SetValue(float64(current))
				statusLabel.SetText(fmt.Sprintf("%s (%d/%d)", message, current, total))

				logData = append(logData, message)
				logList.Refresh()
				logList.ScrollToBottom()
			}

			byDate := groupingRadio.Selected == "Group by Date"
			recursive := false
			if byDate {
				recursive = recursiveCheck.Checked
			} else {
				// Group by Folder implies processing folders
				recursive = true
			}

			err := UploadDirWithProgress(
				selectedPath,
				userId,
				byDate,
				false, // ignoreExif is now always false (use EXIF)
				recursive,
				progressCallback,
				control,
			)

			uploadButton.Enable()
			backButton.Enable()
			cancelButton.Disable()
			pauseButton.Disable()

			if err != nil {
				if err.Error() == "upload cancelled" {
					statusLabel.SetText("Upload cancelled")
					dialog.ShowInformation("Cancelled", "Upload was cancelled by user.", w)
				} else {
					statusLabel.SetText("Upload failed!")
					showError(fmt.Errorf("Upload failed: %v", err), w)
				}
			} else {
				statusLabel.SetText("Upload complete!")
				dialog.ShowInformation("Success", "All photos have been uploaded successfully!", w)
			}
		}()
	})
	uploadButton.Importance = widget.HighImportance

	// Layout
	// Use Padded container for margins
	optionsBox := container.NewVBox(
		widget.NewLabel("Options:"),
		groupingRadio,
		recursiveCheck,
	)

	progressBox := container.NewVBox(
		widget.NewLabel("Progress:"),
		progressBar,
		statusLabel,
	)

	// Combine elements with spacers
	mainContent := container.NewVBox(
		header,
		widget.NewSeparator(),
		pathLabel,
		layout.NewSpacer(),
		optionsBox,
		layout.NewSpacer(),
		progressBox,
		layout.NewSpacer(),
		widget.NewLabel("Log:"),
		logContainer,
		layout.NewSpacer(),
		container.NewHBox(backButton, layout.NewSpacer(), pauseButton, cancelButton, uploadButton),
	)

	w.SetContent(container.NewPadded(container.NewPadded(mainContent)))
}

func showError(err error, w fyne.Window) {
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapBreak

	// Use a scroll container to handle very long messages
	scroll := container.NewVScroll(label)
	scroll.SetMinSize(fyne.NewSize(400, 100))

	dialog.ShowCustom("Error", "OK", scroll, w)
}
