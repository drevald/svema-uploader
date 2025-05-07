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

const BaseUrlDev = "http://valdr:7777/api"
//const BaseUrlDev = "http://localhost:7777/api"

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

func PostShot(shot Shot) {
	shotJson, _ := json.Marshal(shot)
	resp, err := http.Post(BaseUrlDev+"/shots", "application/json", bytes.NewReader(shotJson))
	if err != nil {
		fmt.Println("Request error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
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

func UploadDir(dirname string, userId int, byDate bool, ignoreExif bool) {
	dirs, err := os.ReadDir(dirname)
	if err != nil {
		fmt.Println("Error reading root directory:", err)
		return
	}

	albums := make(map[string]Album)
	filesTotal := 0

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
			if file.IsDir() {
				UploadDir(filepath.Join(filepath.Join(dirname, dir.Name()), file.Name()), userId, byDate, ignoreExif)
			}
			if !(strings.HasSuffix(strings.ToLower(file.Name()), "jpg") ||
				strings.HasSuffix(strings.ToLower(file.Name()), "tiff") ||
				strings.HasSuffix(strings.ToLower(file.Name()), "tif")) {
				continue
			}

			filename := filepath.Join(albumPath, file.Name())
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

func main() {
	fmt.Print("Starting...")

	if len(os.Args) < 3 {
		fmt.Println("Usage: uploader <user_id> <directory> [--by-date] [--ignore-exif]")
		return
	}

	userId, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Conversion error:", err)
		return
	}

	dirname := os.Args[2]
	byDate := len(os.Args) > 3 && os.Args[3] == "--by-date"
	ignoreExif := len(os.Args) > 4 && os.Args[4] == "--ignore-exif"

	UploadDir(dirname, userId, byDate, ignoreExif)
}
