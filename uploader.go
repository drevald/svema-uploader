package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// BaseUrlDev = "http://dobby:7777/api"
	BaseUrlDev = "http://localhost:7777/api"
	// BaseUrlDev = "http://svema.valdr.ru/api"
	// BaseUrlDev = "http://192.168.0.148:7777/api"
)

// type errorResponse struct {
// 	Code int		//'json:"code"'
// 	Message string	//'json:"message"'
// }

type Album struct {
	AlbumId   int    //'json:"albumId"'
	Name      string //'json:"name"'
	UserId    int    //'json:"user"'
	PreviewId int    //'json:"previewId"'
}

type Shot struct {
	ShotId    int
	AlbumId   int
	Name      string //'json:"name"'
	UserId    int    //'json:"user"'
	DateStart string
	DateEnd   string
	Data      []byte
	Mime      string
	OrigPath  string
}

func GetAlbums() []Album {
	resp, err := http.Get(BaseUrlDev + "/albums")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
	albums := []Album{}
	err = json.Unmarshal(body, &albums)
	if err != nil {
		fmt.Println(err)
	}
	return albums
}

func PostAlbum(album Album) Album {
	fmt.Printf("...Creating album %s\n", album.Name)
	albumJson, _ := json.Marshal(album)
	reader := bytes.NewReader(albumJson)
	resp, err := http.Post(BaseUrlDev+"/albums", "application/json", reader)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	updated_album := Album{}
	json.Unmarshal(body, &updated_album)
	return updated_album
}

func getExifDate(imageBytes []byte) (*time.Time, error) {
	// Register manufacturer-specific notes
	exif.RegisterParsers(mknote.All...)

	// Decode EXIF from bytes
	x, err := exif.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("could not decode exif: %w", err)
	}

	// Get the DateTimeOriginal tag specifically
	tag, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		return nil, fmt.Errorf("DateTimeOriginal not found: %w", err)
	}

	rawStr, err := tag.StringVal()
	if err != nil {
		return nil, fmt.Errorf("could not get string value: %w", err)
	}

	// DEBUG
	fmt.Printf("🔍 Raw DateTimeOriginal string: %q\n", rawStr)

	// Parse EXIF format date
	tm, err := time.Parse("2006:01:02 15:04:05", rawStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse datetime: %w", err)
	}

	return &tm, nil
}

func getLastModifiedDate(path string) (*time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime()
	return &modTime, nil
}

func getFileCreationDate(path string) (*time.Time, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)

	var data syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(h, &data)
	if err != nil {
		return nil, err
	}

	ft := data.CreationTime
	t := time.Unix(0, ft.Nanoseconds())
	return &t, nil
}

func getShotCreationDate(path string) (*time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Fail to read %s\n", path)
		return nil, err
	}
	t, err := getExifDate(data)
	if err != nil {
		fmt.Printf("Failed to get EXIF from %s\n", path)
		t, err = getLastModifiedDate(path)
		if err != nil {
			return nil, err
		}
		return t, nil
	}
	return t, nil
}

func PostShot(shot Shot) {
	shotJson, _ := json.Marshal(shot)
	reader := bytes.NewReader(shotJson)

	resp, err := http.Post(BaseUrlDev+"/shots", "application/json", reader)
	if err != nil {
		fmt.Println("Request error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Try to decode error JSON
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			fmt.Println("Server error:", errResp)
		} else {
			fmt.Printf("HTTP %d: %s\n", resp.StatusCode, string(body))
		}
		return
	}

	fmt.Println("Shot uploaded successfully.")
}

///////////////////////////////////////////////

func main() {

	fmt.Print("Starting...")

	if len(os.Args) < 3 {
		fmt.Println("Usage: uploader <user_id> <directory> [--by-date]")
		return
	}

	userId, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Conversion error:", err)
		return
	}

	dirname := os.Args[2]
	byDate := len(os.Args) > 3 && os.Args[3] == "--by-date"

	dirs, err := os.ReadDir(dirname)
	if err != nil {
		fmt.Println("Error reading root directory:", err)
		return
	}

	albums := make(map[string]Album)

	filesTotal := 0

	for _, entry := range dirs {
		fmt.Println(entry.Name()) // safer than fmt.Print(entry)

		if entry.IsDir() {
			subdirPath := filepath.Join(dirname, entry.Name()) // full path to subdirectory
			files, err := os.ReadDir(subdirPath)
			if err != nil {
				fmt.Println("Error reading subdir:", err)
				continue
			}

			for _, file := range files {
				if !file.IsDir() {
					filesTotal++
				}
			}
		}
	}

	filesSent := 0

	for _, dir := range dirs {

		if !dir.IsDir() {
			continue
		}

		albumName := dir.Name()
		albumDir := filepath.Join(dirname, albumName)
		files, err := os.ReadDir(albumDir)
		if err != nil {
			fmt.Printf("Error reading album folder %s: %v\n", albumName, err)
			continue
		}

		// Default date range from album name
		dateStart := "1874-07-24"
		dateEnd := "1874-07-24"
		parts := strings.Split(albumName, "_")
		if len(parts) > 1 && len(parts[1]) == 4 {
			if num, err := strconv.Atoi(parts[1]); err == nil {
				dateStart = fmt.Sprintf("%d-01-01T00:00:00Z", num)
				dateEnd = fmt.Sprintf("%d-12-31T00:00:00Z", num)
			}
		} else if len(parts) > 1 && len(parts[1]) >= 3 {
			decade := parts[1][:3]
			if num, err := strconv.Atoi(decade); err == nil {
				dateStart = fmt.Sprintf("%d0-01-01T00:00:00Z", num)
				dateEnd = fmt.Sprintf("%d9-12-31T00:00:00Z", num)
			}
		}

		// Pre-create album by default (in no --by-date mode)
		defaultAlbum := Album{Name: albumName, UserId: userId, PreviewId: 0}
		if !byDate {
			defaultAlbum = PostAlbum(defaultAlbum)
			albums[albumName] = defaultAlbum
		}

		for _, file := range files {

			// Corrected code
			if file.IsDir() ||
				(!strings.HasSuffix(strings.ToLower(file.Name()), "jpg") &&
					!strings.HasSuffix(strings.ToLower(file.Name()), "tiff") &&
					!strings.HasSuffix(strings.ToLower(file.Name()), "tif")) {
				continue
			}

			filename := filepath.Join(albumDir, file.Name())
			fmt.Println("Processing:", filename)
			data, err := os.ReadFile(filename)
			if err != nil {
				fmt.Println("Error reading file:", err)
				continue
			}

			ext := filepath.Ext(file.Name())
			var targetAlbum Album

			if byDate {
				fmt.Println("Upload by date")
				if t, err := getShotCreationDate(filename); err == nil && t != nil {
					dateStart = t.Format(time.RFC3339)
					dateEnd = t.Format(time.RFC3339)
					dateKey := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())
					storedAlbum, exists := albums[dateKey]
					if !exists {
						newAlbum := Album{Name: dateKey, UserId: userId, PreviewId: 0}
						storedAlbum = PostAlbum(newAlbum)
						albums[dateKey] = storedAlbum
					}
					targetAlbum = storedAlbum
				} else {
					fmt.Println("Could not extract EXIF date, using fallback album")
					targetAlbum = defaultAlbum
				}
			} else {
				fmt.Print("Upload by dir")
				targetAlbum = defaultAlbum
			}

			shot := Shot{
				AlbumId:   targetAlbum.AlbumId,
				Name:      file.Name(),
				UserId:    userId,
				DateStart: dateStart,
				DateEnd:   dateEnd,
				Data:      data,
				Mime:      mime.TypeByExtension(ext),
				OrigPath:  filename,
			}

			PostShot(shot)
			filesSent++
			fmt.Printf("%d of %d sent\n", filesSent, filesTotal)
		}
	}
}
