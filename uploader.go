package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)

//const BaseUrlDev = "http://svema.valdr.ru/api"

const BaseUrlDev = "http://localhost:7777/api"

type Album struct {
	AlbumId   int
	Name      string
	UserId    int
	PreviewId int
}

type Shot struct {
	ShotId    int
	AlbumId   int
	Name      string
	UserId    int
	DateStart string
	DateEnd   string
	Data      []byte
	Mime      string
	OrigPath  string
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type UploadControl struct {
	mu        sync.Mutex
	paused    bool
	cancelled bool
}

func (c *UploadControl) IsPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

func (c *UploadControl) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = paused
}

func (c *UploadControl) IsCancelled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelled
}

func (c *UploadControl) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelled = true
}

type UploadResponse struct {
	Message string  `json:"message"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Date    string  `json:"date"`
	Album   string  `json:"album"`
}

func GetAlbums() []Album {
	resp, err := http.Get(BaseUrlDev + "/albums")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	var albums []Album
	if err := json.Unmarshal(body, &albums); err != nil {
		fmt.Println(err)
	}
	return albums
}

func PostAlbum(album Album) Album {
	fmt.Printf("...Creating album %s\n", album.Name)
	albumJson, _ := json.Marshal(album)
	resp, err := http.Post(BaseUrlDev+"/albums", "application/json", bytes.NewReader(albumJson))
	if err != nil {
		fmt.Println(err)
		return album
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var updated Album
	json.Unmarshal(body, &updated)
	return updated
}

func getExifDate(imageBytes []byte) (*time.Time, error) {
	exif.RegisterParsers(mknote.All...)

	x, err := exif.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("could not decode exif: %w", err)
	}

	tag, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		return nil, fmt.Errorf("DateTimeOriginal not found: %w", err)
	}

	rawStr, err := tag.StringVal()
	if err != nil {
		return nil, fmt.Errorf("could not get string value: %w", err)
	}

	fmt.Printf("🔍 Raw DateTimeOriginal string: %q\n", rawStr)

	tm, err := time.Parse("2006:01:02 15:04:05", rawStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse datetime: %w", err)
	}

	return &tm, nil
}

func getGPSCoordinates(imageBytes []byte) (lat, lon *float64) {
	exif.RegisterParsers(mknote.All...)

	x, err := exif.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, nil
	}

	latVal, lonVal, err := x.LatLong()
	if err != nil {
		return nil, nil
	}

	// Round to 2 decimal places
	latRounded := float64(int(latVal*100+0.5)) / 100
	lonRounded := float64(int(lonVal*100+0.5)) / 100

	return &latRounded, &lonRounded
}

func getLastModifiedDate(path string) (*time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	t := info.ModTime()
	return &t, nil
}

func getFileCreationDate(path string) (*time.Time, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)

	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &data); err != nil {
		return nil, err
	}

	t := time.Unix(0, data.CreationTime.Nanoseconds())
	return &t, nil
}

func getShotCreationDate(path string, ignoreExif bool) (*time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Fail to read %s\n", path)
		return nil, err
	}

	t, err := getExifDate(data)
	if err != nil || ignoreExif {
		fmt.Printf("Failed to get EXIF from %s\n", path)
		return getLastModifiedDate(path)
	}
	return t, nil
}

func PostShot(shot Shot) (*UploadResponse, error) {
	shotJson, _ := json.Marshal(shot)
	resp, err := http.Post(BaseUrlDev+"/shots", "application/json", bytes.NewReader(shotJson))
	if err != nil {
		fmt.Println("Request error:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			fmt.Println("Server error:", errResp)
			return nil, fmt.Errorf("server error: %v", errResp)
		} else {
			fmt.Printf("HTTP %d: %s\n", resp.StatusCode, string(body))
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
	}

	// Parse the response
	var uploadResp UploadResponse
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		fmt.Println("Shot uploaded successfully (couldn't parse response).")
		return nil, nil
	}

	fmt.Println("Shot uploaded successfully.")
	return &uploadResp, nil
}

type ProgressCallback func(current, total int, message string)

func UploadDir(dirname string, userId int, byDate bool, ignoreExif bool) error {
	return UploadDirWithProgress(dirname, userId, byDate, ignoreExif, nil, nil)
}

func UploadDirWithProgress(dirname string, userId int, byDate bool, ignoreExif bool, progressCallback ProgressCallback, control *UploadControl) error {
	dirs, err := os.ReadDir(dirname)
	if err != nil {
		fmt.Println("Error reading root directory:", err)
		return err
	}

	albums := make(map[string]Album)
	filesTotal := 0
	failures := 0

	// Count files in subdirectories
	for _, entry := range dirs {
		if entry.IsDir() {
			subdir := filepath.Join(dirname, entry.Name())
			files, err := os.ReadDir(subdir)
			if err != nil {
				fmt.Println("Error reading subdir:", err)
				continue
			}
			for _, file := range files {
				if !file.IsDir() {
					ext := strings.ToLower(filepath.Ext(file.Name()))
					if ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif" {
						filesTotal++
					}
				}
			}
		} else {
			// Count files in root directory
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif" {
				filesTotal++
			}
		}
	}

	if filesTotal == 0 {
		return fmt.Errorf("no images found in directory (checked for .jpg, .jpeg, .tiff, .tif)")
	}

	filesSent := 0

	// Process files in root directory first
	rootFiles, err := os.ReadDir(dirname)
	if err == nil {
		// Create a default album for root files using the directory name
		rootAlbumName := filepath.Base(dirname)
		var rootAlbum Album
		rootAlbumCreated := false

		for _, file := range rootFiles {
			if control != nil {
				if control.IsCancelled() {
					return fmt.Errorf("upload cancelled")
				}
				for control.IsPaused() {
					if control.IsCancelled() {
						return fmt.Errorf("upload cancelled")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}

			if file.IsDir() {
				continue
			}

			ext := strings.ToLower(filepath.Ext(file.Name()))
			if !(ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif") {
				continue
			}

			if !rootAlbumCreated {
				// Initialize root album if we have files to upload
				defaultAlbum := Album{Name: rootAlbumName, UserId: userId}
				if !byDate {
					defaultAlbum = PostAlbum(defaultAlbum)
					albums[rootAlbumName] = defaultAlbum
				}
				rootAlbum = defaultAlbum
				rootAlbumCreated = true
			}

			// Upload logic for root file
			filename := filepath.Join(dirname, file.Name())
			fmt.Println("Processing root file:", filename)

			data, err := os.ReadFile(filename)
			if err != nil {
				fmt.Println("Error reading file:", err)
				continue
			}

			var targetAlbum Album
			dateStart := "1874-07-24"
			dateEnd := "1874-07-24"

			if byDate {
				if t, err := getShotCreationDate(filename, ignoreExif); err == nil && t != nil {
					dateStart = t.Format(time.RFC3339)
					dateEnd = t.Format(time.RFC3339)
					dateKey := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())
					storedAlbum, exists := albums[dateKey]
					if !exists {
						newAlbum := Album{Name: dateKey, UserId: userId}
						storedAlbum = PostAlbum(newAlbum)
						albums[dateKey] = storedAlbum
					}
					targetAlbum = storedAlbum
				} else {
					targetAlbum = rootAlbum
				}
			} else {
				targetAlbum = rootAlbum
			}

			// Extract GPS coordinates
			lat, lon := getGPSCoordinates(data)

			shot := Shot{
				AlbumId:   targetAlbum.AlbumId,
				Name:      file.Name(),
				UserId:    userId,
				DateStart: dateStart,
				DateEnd:   dateEnd,
				Data:      data,
				Mime:      mime.TypeByExtension(ext),
				OrigPath:  filename,
				Latitude:  lat,
				Longitude: lon,
			}

			uploadResp, err := PostShot(shot)
			filesSent++
			if progressCallback != nil {
				var message string
				if err != nil {
					failures++
					message = fmt.Sprintf("Failed|%s|Album: %s|Date: %s|Error: %v", file.Name(), targetAlbum.Name, dateStart, err)
				} else if uploadResp != nil {
					// Use response from server
					gpsInfo := ""
					if uploadResp.Lat != 0 || uploadResp.Lon != 0 {
						gpsInfo = fmt.Sprintf("|GPS: %.2f, %.2f", uploadResp.Lat, uploadResp.Lon)
					}
					message = fmt.Sprintf("Uploaded|%s|Album: %s|Date: %s%s", file.Name(), uploadResp.Album, uploadResp.Date, gpsInfo)
				} else {
					message = fmt.Sprintf("Uploaded|%s|Album: %s|Date: %s", file.Name(), targetAlbum.Name, dateStart)
				}
				progressCallback(filesSent, filesTotal, message)
			}
		}
	}

	// Process subdirectories
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		albumName := dir.Name()
		albumPath := filepath.Join(dirname, albumName)
		files, err := os.ReadDir(albumPath)
		if err != nil {
			fmt.Printf("Error reading album folder %s: %v\n", albumName, err)
			continue
		}

		// Guess date range from folder name
		dateStart := "1874-07-24"
		dateEnd := "1874-07-24"
		parts := strings.Split(albumName, "_")
		if len(parts) > 1 && len(parts[1]) == 4 {
			if year, err := strconv.Atoi(parts[1]); err == nil {
				dateStart = fmt.Sprintf("%d-01-01T00:00:00Z", year)
				dateEnd = fmt.Sprintf("%d-12-31T00:00:00Z", year)
			}
		} else if len(parts) > 1 && len(parts[1]) >= 3 {
			if decade, err := strconv.Atoi(parts[1][:3]); err == nil {
				dateStart = fmt.Sprintf("%d0-01-01T00:00:00Z", decade)
				dateEnd = fmt.Sprintf("%d9-12-31T00:00:00Z", decade)
			}
		}

		defaultAlbum := Album{Name: albumName, UserId: userId}
		if !byDate {
			defaultAlbum = PostAlbum(defaultAlbum)
			albums[albumName] = defaultAlbum
		}

		for _, file := range files {
			if control != nil {
				if control.IsCancelled() {
					return fmt.Errorf("upload cancelled")
				}
				for control.IsPaused() {
					if control.IsCancelled() {
						return fmt.Errorf("upload cancelled")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}

			if file.IsDir() {
				UploadDirWithProgress(filepath.Join(filepath.Join(dirname, dir.Name()), file.Name()), userId, byDate, ignoreExif, progressCallback, control)
			}
			ext := strings.ToLower(filepath.Ext(file.Name()))
			if !(ext == ".jpg" || ext == ".jpeg" || ext == ".tiff" || ext == ".tif") {
				continue
			}

			filename := filepath.Join(albumPath, file.Name())
			fmt.Println("Processing:", filename)

			data, err := os.ReadFile(filename)
			if err != nil {
				fmt.Println("Error reading file:", err)
				continue
			}

			ext = filepath.Ext(file.Name())
			var targetAlbum Album

			if byDate {
				fmt.Println("Upload by date")
				if t, err := getShotCreationDate(filename, ignoreExif); err == nil && t != nil {
					dateStart = t.Format(time.RFC3339)
					dateEnd = t.Format(time.RFC3339)
					dateKey := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())
					storedAlbum, exists := albums[dateKey]
					if !exists {
						newAlbum := Album{Name: dateKey, UserId: userId}
						storedAlbum = PostAlbum(newAlbum)
						albums[dateKey] = storedAlbum
					}
					targetAlbum = storedAlbum
				} else {
					fmt.Println("Could not extract EXIF date, using fallback album")
					targetAlbum = defaultAlbum
				}
			} else {
				fmt.Println("Upload by dir")
				targetAlbum = defaultAlbum
			}

			// Extract GPS coordinates
			lat, lon := getGPSCoordinates(data)

			shot := Shot{
				AlbumId:   targetAlbum.AlbumId,
				Name:      file.Name(),
				UserId:    userId,
				DateStart: dateStart,
				DateEnd:   dateEnd,
				Data:      data,
				Mime:      mime.TypeByExtension(ext),
				OrigPath:  filename,
				Latitude:  lat,
				Longitude: lon,
			}

			uploadResp, err := PostShot(shot)
			filesSent++
			fmt.Printf("%d of %d sent\n", filesSent, filesTotal)

			if progressCallback != nil {
				var message string
				if err != nil {
					failures++
					message = fmt.Sprintf("Failed|%s|Album: %s|Date: %s|Error: %v", file.Name(), targetAlbum.Name, dateStart, err)
				} else if uploadResp != nil {
					// Use response from server
					gpsInfo := ""
					if uploadResp.Lat != 0 || uploadResp.Lon != 0 {
						gpsInfo = fmt.Sprintf("|GPS: %.2f, %.2f", uploadResp.Lat, uploadResp.Lon)
					}
					message = fmt.Sprintf("Uploaded|%s|Album: %s|Date: %s%s", file.Name(), uploadResp.Album, uploadResp.Date, gpsInfo)
				} else {
					message = fmt.Sprintf("Uploaded|%s|Album: %s|Date: %s", file.Name(), targetAlbum.Name, dateStart)
				}
				progressCallback(filesSent, filesTotal, message)
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf("upload completed with %d failures", failures)
	}
	return nil
}

// CLI entry point - uncomment if you want to build a CLI version
// func main() {
// 	fmt.Print("Starting...")
//
// 	if len(os.Args) < 3 {
// 		fmt.Println("Usage: uploader <user_id> <directory> [--by-date] [--ignore-exif]")
// 		return
// 	}
//
// 	userId, err := strconv.Atoi(os.Args[1])
// 	if err != nil {
// 		fmt.Println("Conversion error:", err)
// 		return
// 	}
//
// 	dirname := os.Args[2]
// 	byDate := len(os.Args) > 3 && os.Args[3] == "--by-date"
// 	ignoreExif := len(os.Args) > 4 && os.Args[4] == "--ignore-exif"
//
// 	UploadDir(dirname, userId, byDate, ignoreExif)
// }
