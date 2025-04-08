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
	
	dirname := os.Args[1]

	dirs, err := os.ReadDir(dirname)
    if err != nil {
        fmt.Print(err)
    }

    for _, dir := range dirs {

		if (dir.IsDir()) {
			album := Album {
				Name: dir.Name(),
				UserId: 1,
				PreviewId: 0,
			}

			stored_album := PostAlbum(album)	
			albumdirname := dirname + "\\" + dir.Name()
			files, err := os.ReadDir(albumdirname)
			if err != nil {
				fmt.Print(err)
			}

			dateStart := "1874-07-24"
			dateEnd := "1874-07-24"

			parts := strings.Split(album.Name, "_")
			 
			if (len(parts) > 1 && len([]rune(parts[1])) == 4) {
				if num, err := strconv.Atoi(parts[1]); err == nil {
						dateStart = fmt.Sprintf("%d%s", num, "-01-01")
						dateEnd = fmt.Sprintf("%d%s", num, "-12-31")
					} else {
					decade := parts[1][0:3]
					fmt.Printf("\n%s\n", decade)
					if num, err := strconv.Atoi(decade); err == nil {
						dateStart = fmt.Sprintf("%d%s", num, "0-01-01")
						dateEnd = fmt.Sprintf("%d%s", num, "9-12-31")						
						fmt.Printf("%d\n", num)
					}
				}
			}

			for _, file := range files {
				ext := filepath.Ext(file.Name())
				filename := albumdirname + "\\" + file.Name()
				fmt.Printf("%s\n", filename)
				bytes, _ := os.ReadFile(filename)
				shot := Shot {
					AlbumId: stored_album.AlbumId,
					Name: file.Name(),
					DateStart: dateStart,
					DateEnd: dateEnd,
					Data: bytes,
					Mime: mime.TypeByExtension(ext),
					UserId: 1,
				}
				PostShot(shot)
			}

			fmt.Printf("%d\n", stored_album.AlbumId)

		}
    }

}