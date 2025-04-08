package main

import (
	"net/http"
	"fmt"
	"io"
	"encoding/json"
	"bytes"
	"os"
	"strings"
	"strconv"
	"mime"
	"path/filepath"
    "github.com/rwcarlsen/goexif/exif"
    "github.com/rwcarlsen/goexif/mknote"	
	"time"
)

const (
	//BaseUrlDev = "http://dobby:7777/api"
	BaseUrlDev = "http://localhost:8888/api"
	//BaseUrlDev = "http://svema.valdr.ru/api"
	//BaseUrlDev = "http://192.168.0.148:7777/api"
)

// type errorResponse struct {
// 	Code int		//'json:"code"'
// 	Message string	//'json:"message"'
// }

type Album struct {
	AlbumId int		//'json:"albumId"'
	Name string		//'json:"name"'
	UserId int		//'json:"user"'
	PreviewId int	//'json:"previewId"'
}

type Shot struct {
	ShotId int			
	AlbumId int
	Name string		//'json:"name"'
	UserId int		//'json:"user"'
	DateStart string
	DateEnd string
	Data []byte
	Mime string
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
	if (err != nil) {
		fmt.Println(err)
	}
	return albums
}

func PostAlbum(album Album) Album {
	albumJson, _ := json.Marshal(album)
	reader := bytes.NewReader(albumJson)
	resp, err := http.Post(BaseUrlDev + "/albums", "application/json", reader)
	if (err != nil) {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)	
	updated_album := Album{}
	json.Unmarshal(body, &updated_album)	
	return updated_album
}

func getDateTimeOriginal(imageBytes []byte) (*time.Time, error) {
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

func PostShot(shot Shot) {
	shotJson, _ := json.Marshal(shot)
	reader := bytes.NewReader(shotJson)
	_, err := http.Post(BaseUrlDev + "/shots", "application/json", reader)
	if (err != nil) {
		fmt.Println(err)
	}
}

func main() {

	fmt.Print("Starting...")

	if len(os.Args) < 2 {
		fmt.Println("Usage: uploader <directory> [--by-date]")
		return
	}

	dirname := os.Args[1]
	byDate := len(os.Args) > 2 && os.Args[2] == "--by-date"

	dirs, err := os.ReadDir(dirname)
	if err != nil {
		fmt.Println("Error reading root directory:", err)
		return
	}

	albums := make(map[string]Album)


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
				dateStart = fmt.Sprintf("%d-01-01", num)
				dateEnd = fmt.Sprintf("%d-12-31", num)
			}
		} else if len(parts) > 1 && len(parts[1]) >= 3 {
			decade := parts[1][:3]
			if num, err := strconv.Atoi(decade); err == nil {
				dateStart = fmt.Sprintf("%d0-01-01", num)
				dateEnd = fmt.Sprintf("%d9-12-31", num)
			}
		}

		// Pre-create album by default (in no --by-date mode)
		defaultAlbum := Album{Name: albumName, UserId: 1, PreviewId: 0}
		defaultAlbum = PostAlbum(defaultAlbum)
		albums[albumName] = defaultAlbum

		for _, file := range files {

			if file.IsDir() {
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
				fmt.Print("Upload by date")
				if t, err := getDateTimeOriginal(data); err == nil && t != nil {
					dateKey := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())
					storedAlbum, exists := albums[dateKey]
					if !exists {
						newAlbum := Album{Name: dateKey, UserId: 1, PreviewId: 0}
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
				AlbumId:  targetAlbum.AlbumId,
				Name:     file.Name(),
				UserId:   1,
				DateStart: dateStart,
				DateEnd:   dateEnd,
				Data:     data,
				Mime:     mime.TypeByExtension(ext),
			}

			PostShot(shot)
		}
	}
}