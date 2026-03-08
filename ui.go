package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/rwcarlsen/goexif/exif"
)

func showForgotPasswordScreen(w fyne.Window, a fyne.App) {
	header := widget.NewLabelWithStyle("Сброс пароля", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	hint := widget.NewLabel("Введите ваш email и мы отправим ссылку для сброса пароля.")
	hint.Wrapping = fyne.TextWrapWord

	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("Введите email")

	submitButton := widget.NewButton("Отправить ссылку", func() {
		email := strings.TrimSpace(emailEntry.Text)
		if email == "" {
			showError(fmt.Errorf("Пожалуйста, введите email"), w)
			return
		}
		err := RequestPasswordReset(email)
		if err != nil {
			showError(fmt.Errorf("Не удалось отправить ссылку: %v", err), w)
			return
		}
		dialog.ShowInformation("Письмо отправлено",
			"Если аккаунт с таким email существует, ссылка для сброса пароля отправлена.",
			w)
		showLoginScreen(w, a)
	})

	backButton := widget.NewButton("Назад к входу", func() {
		showLoginScreen(w, a)
	})

	content := container.NewVBox(
		widget.NewLabel(""),
		header,
		widget.NewLabel(""),
		hint,
		widget.NewLabel(""),
		widget.NewLabel("Email:"),
		emailEntry,
		widget.NewLabel(""),
		submitButton,
		backButton,
	)

	w.SetContent(container.NewCenter(content))
}

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

// extractExifThumbnail attempts to extract the embedded EXIF thumbnail from a JPEG file.
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

	return x.JpegThumbnail()
}

func showFileBrowser(w fyne.Window, a fyne.App, userId int, currentPath string) {
	pathLabel := widget.NewLabel("Current: " + currentPath)
	pathLabel.Wrapping = fyne.TextWrapBreak

	upButton := widget.NewButtonWithIcon("Вверх", theme.NavigateBackIcon(), func() {
		parent := filepath.Dir(currentPath)
		if parent != currentPath {
			showFileBrowser(w, a, userId, parent)
		}
	})

	homeButton := widget.NewButtonWithIcon("Домой", theme.HomeIcon(), func() {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			showFileBrowser(w, a, userId, homeDir)
		}
	})

	// Build list of available drives (Windows only; no-op on other platforms)
	var driveOptions []string
	currentDrive := ""
	for drive := 'A'; drive <= 'Z'; drive++ {
		drivePath := string(drive) + ":\\"
		if _, err := os.Stat(drivePath); err == nil {
			driveOptions = append(driveOptions, string(drive)+":")
			if strings.HasPrefix(strings.ToUpper(currentPath), string(drive)+":") {
				currentDrive = string(drive) + ":"
			}
		}
	}

	driveSelect := widget.NewSelect(driveOptions, func(selected string) {
		if selected != currentDrive {
			showFileBrowser(w, a, userId, selected+"\\")
		}
	})
	if currentDrive != "" {
		driveSelect.SetSelected(currentDrive)
	}
	driveSelect.PlaceHolder = "Диск"
	driveSelectContainer := container.NewGridWrap(fyne.NewSize(80, 36), driveSelect)

	selectButton := widget.NewButton("Выбрать эту папку", func() {
		saveRecentDir(currentPath)
		showUploadConfig(w, a, userId, currentPath)
	})
	selectButton.Importance = widget.HighImportance

	files, err := os.ReadDir(currentPath)
	if err != nil {
		showError(err, w)
		return
	}

	var folders []os.DirEntry
	var filesList []os.DirEntry
	for _, f := range files {
		if f.IsDir() && strings.HasPrefix(f.Name(), ".") {
			continue
		}
		if f.IsDir() {
			folders = append(folders, f)
		} else {
			filesList = append(filesList, f)
		}
	}

	gridContent := container.NewGridWithColumns(4)

	for _, f := range folders {
		name := f.Name()
		path := filepath.Join(currentPath, name)
		btn := widget.NewButtonWithIcon(name, theme.FolderIcon(), func() {
			showFileBrowser(w, a, userId, path)
		})
		if len(name) > 15 {
			btn.SetText(name[:12] + "...")
		}
		gridContent.Add(btn)
	}

	imageCount := 0
	for _, f := range filesList {
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tiff" || ext == ".tif" {
			imageCount++
		}
	}

	loadThumbnails := false

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

		imgHolder := container.NewStack()

		icon := theme.FileIcon()
		if isImage {
			icon = theme.FileImageIcon()
		}

		imgWidget := canvas.NewImageFromResource(icon)
		imgWidget.FillMode = canvas.ImageFillContain
		imgWidget.SetMinSize(fyne.NewSize(100, 100))
		imgHolder.Add(imgWidget)

		label := widget.NewLabel(name)
		label.Alignment = fyne.TextAlignCenter
		label.Wrapping = fyne.TextTruncate

		gridContent.Add(container.NewVBox(imgHolder, label))

		if isImage && loadThumbnails {
			jobs <- thumbnailJob{path: path, imgWidget: imgWidget}
		}
	}

	if loadThumbnails && jobs != nil {
		close(jobs)
		numWorkers := 4
		for i := 0; i < numWorkers; i++ {
			go func() {
				for job := range jobs {
					data, err := extractExifThumbnail(job.path)
					if err != nil {
						data, err = os.ReadFile(job.path)
						if err != nil {
							continue
						}
					}
					res := fyne.NewStaticResource(filepath.Base(job.path), data)
					job.imgWidget.Resource = res
					job.imgWidget.Refresh()
				}
			}()
		}
	}

	scroll := container.NewVScroll(gridContent)

	logoutButton := widget.NewButton("Выйти", func() {
		showLoginScreen(w, a)
	})

	topBar := container.NewBorder(nil, nil, container.NewHBox(upButton, homeButton, driveSelectContainer), logoutButton, pathLabel)

	recentDirs := getRecentDirs()
	var mainContent fyne.CanvasObject

	if len(recentDirs) > 0 {
		recentLabel := widget.NewLabel("Недавние:")
		recentLabel.TextStyle = fyne.TextStyle{Bold: true}

		recentBtns := []fyne.CanvasObject{recentLabel}
		for i, dir := range recentDirs {
			if i >= 3 {
				break
			}
			if _, err := os.Stat(dir); err == nil {
				dirPath := dir
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
			selectButton, nil, nil, scroll,
		)
	} else {
		mainContent = container.NewBorder(topBar, selectButton, nil, nil, scroll)
	}

	w.SetContent(mainContent)
}

func showUploadConfig(w fyne.Window, a fyne.App, userId int, selectedPath string) {
	header := widget.NewLabelWithStyle("Настройки загрузки", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	pathLabel := widget.NewLabel("Папка: " + selectedPath)
	pathLabel.Wrapping = fyne.TextWrapBreak

	groupingRadio := widget.NewRadioGroup([]string{"Группировать по дате", "Группировать по папке"}, nil)
	groupingRadio.Horizontal = true
	groupingRadio.Selected = "Группировать по дате"

	recursiveCheck := widget.NewCheck("Обрабатывать подпапки рекурсивно", nil)
	recursiveCheck.Checked = false

	groupingRadio.OnChanged = func(selected string) {
		if selected == "Группировать по дате" {
			recursiveCheck.Enable()
		} else {
			recursiveCheck.Disable()
			recursiveCheck.SetChecked(true)
		}
	}

	var (
		barcodeStatuses []int8
		barcodeTotal    int
		barcodeMu       sync.Mutex
	)

	barcodeRaster := canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		barcodeMu.Lock()
		defer barcodeMu.Unlock()
		if barcodeTotal == 0 || w == 0 {
			return color.NRGBA{R: 210, G: 210, B: 210, A: 255}
		}
		idx := x * barcodeTotal / w
		if idx < 0 || idx >= len(barcodeStatuses) {
			return color.NRGBA{R: 210, G: 210, B: 210, A: 255}
		}
		switch barcodeStatuses[idx] {
		case 1:
			return color.NRGBA{R: 60, G: 190, B: 60, A: 255}
		case 2:
			return color.NRGBA{R: 220, G: 50, B: 50, A: 255}
		default:
			return color.NRGBA{R: 210, G: 210, B: 210, A: 255}
		}
	})
	barcodeRaster.SetMinSize(fyne.NewSize(0, 32))

	statusLabel := widget.NewLabel("")

	logData := []string{}
	logList := widget.NewList(
		func() int { return len(logData) },
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapWord
			return container.NewStack(label)
		},
		func(i int, o fyne.CanvasObject) {
			stack := o.(*fyne.Container)
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
					if strings.HasPrefix(parts[4], "GPS:") {
						gpsInfo = parts[4]
						if len(parts) >= 6 {
							errorInfo = parts[5]
						}
					} else if strings.HasPrefix(parts[4], "Error:") {
						errorInfo = parts[4]
					} else {
						gpsInfo = parts[4]
					}
				}

				if status == "Failed" {
					bgColor = color.NRGBA{R: 255, G: 200, B: 200, A: 255}
					displayText = fmt.Sprintf("%s - %s - %s", filename, albumInfo, dateInfo)
					if gpsInfo != "" {
						displayText += fmt.Sprintf(" - %s", gpsInfo)
					}
					if errorInfo != "" {
						displayText += fmt.Sprintf(" - %s", errorInfo)
					}
				} else {
					bgColor = color.NRGBA{R: 200, G: 255, B: 200, A: 255}
					displayText = fmt.Sprintf("%s - %s - %s", filename, albumInfo, dateInfo)
					if gpsInfo != "" {
						displayText += fmt.Sprintf(" - %s", gpsInfo)
					}
				}
			} else {
				displayText = msg
				if strings.HasPrefix(msg, "Failed") {
					bgColor = color.NRGBA{R: 255, G: 200, B: 200, A: 255}
				} else {
					bgColor = color.NRGBA{R: 200, G: 255, B: 200, A: 255}
				}
			}

			bg := canvas.NewRectangle(bgColor)
			label := widget.NewLabel(displayText)
			label.Wrapping = fyne.TextWrapWord
			label.TextStyle = fyne.TextStyle{}
			stack.Objects = []fyne.CanvasObject{bg, container.NewPadded(label)}
			stack.Refresh()
		},
	)
	logContainer := container.NewVScroll(logList)
	logContainer.SetMinSize(fyne.NewSize(0, 150))

	backButton := widget.NewButton("Назад", func() {
		showFileBrowser(w, a, userId, selectedPath)
	})

	var uploadButton *widget.Button
	var cancelButton *widget.Button
	var pauseButton *widget.Button
	var control *UploadControl

	cancelButton = widget.NewButton("Отмена", func() {
		if control != nil {
			control.Cancel()
			statusLabel.SetText("Отмена...")
			cancelButton.Disable()
			pauseButton.Disable()
		}
	})
	cancelButton.Disable()
	cancelButton.Importance = widget.DangerImportance

	pauseButton = widget.NewButton("Пауза", nil)
	pauseButton.OnTapped = func() {
		if control != nil {
			if control.IsPaused() {
				control.SetPaused(false)
				pauseButton.SetText("Пауза")
				statusLabel.SetText("Продолжаем...")
			} else {
				control.SetPaused(true)
				pauseButton.SetText("Продолжить")
				statusLabel.SetText("Пауза")
			}
		}
	}
	pauseButton.Disable()

	uploadButton = widget.NewButton("Начать загрузку", func() {
		uploadButton.Disable()
		backButton.Disable()
		cancelButton.Enable()
		pauseButton.Enable()
		pauseButton.SetText("Пауза")

		barcodeMu.Lock()
		barcodeStatuses = nil
		barcodeTotal = 0
		barcodeMu.Unlock()
		barcodeRaster.Refresh()

		statusLabel.SetText("Начинаем загрузку...")
		logData = []string{}
		logList.Refresh()

		control = &UploadControl{}

		go func() {
			var uploadedCount, failedCount int
			progressCallback := func(current, total int, message string) {
				barcodeMu.Lock()
				if barcodeTotal == 0 {
					barcodeTotal = total
					barcodeStatuses = make([]int8, total)
				}
				if current > 0 && current <= len(barcodeStatuses) {
					if strings.HasPrefix(message, "Failed") {
						barcodeStatuses[current-1] = 2
						failedCount++
					} else {
						barcodeStatuses[current-1] = 1
						uploadedCount++
					}
				}
				barcodeMu.Unlock()
				barcodeRaster.Refresh()

				statusLabel.SetText(fmt.Sprintf("Загружено %d/%d, ошибок: %d", uploadedCount, total, failedCount))
				logData = append(logData, message)
				logList.Refresh()
				logList.ScrollToBottom()
			}

			byDate := groupingRadio.Selected == "Группировать по дате"
			recursive := false
			if byDate {
				recursive = recursiveCheck.Checked
			} else {
				recursive = true
			}

			err := UploadDirWithProgress(
				selectedPath,
				userId,
				byDate,
				false,
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
					statusLabel.SetText("Загрузка отменена")
					dialog.ShowInformation("Отменено", "Загрузка отменена пользователем.", w)
				} else {
					statusLabel.SetText("Ошибка загрузки!")
					showError(fmt.Errorf("Ошибка загрузки: %v", err), w)
				}
			} else {
				statusLabel.SetText("Загрузка завершена!")
				dialog.ShowInformation("Успешно", "Все фотографии успешно загружены!", w)
			}
		}()
	})
	uploadButton.Importance = widget.HighImportance

	optionsBox := container.NewVBox(
		widget.NewLabel("Параметры:"),
		groupingRadio,
		recursiveCheck,
	)

	progressBox := container.NewVBox(
		widget.NewLabel("Прогресс:"),
		barcodeRaster,
		statusLabel,
	)

	mainContent := container.NewVBox(
		header,
		widget.NewSeparator(),
		pathLabel,
		layout.NewSpacer(),
		optionsBox,
		layout.NewSpacer(),
		progressBox,
		layout.NewSpacer(),
		widget.NewLabel("Журнал:"),
		logContainer,
		layout.NewSpacer(),
		container.NewHBox(backButton, layout.NewSpacer(), pauseButton, cancelButton, uploadButton),
	)

	w.SetContent(container.NewPadded(container.NewPadded(mainContent)))
}

func showError(err error, w fyne.Window) {
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapBreak

	scroll := container.NewVScroll(label)
	scroll.SetMinSize(fyne.NewSize(400, 100))

	dialog.ShowCustom("Error", "OK", scroll, w)
}
