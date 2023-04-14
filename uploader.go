package main

import (
	"net/http"
	"fmt"
	"io/ioutil"
	"strings"
	"encoding/json"
)

const (
	BaseUrlDev = "http://localhost:7777/api"
)

type errorResponse struct {
	Code int		//'json:"code"'
	Message string	//'json:"message"'
}

type Album struct {
	AlbumId int		//'json:"albumId"'
	Name string		//'json:"name"'
	User int		//'json:"user"'
	PreviewId int	//'json:"previewId"'
}

type Shot struct {
	Id int			//'json:"albumId"'
	Name string		//'json:"name"'
	UserId int		//'json:"user"'
	PreviewId int	//'json:"previewId"'
	Data []byte
}

func GetAlbums() []Album {
	resp, err := http.Get(BaseUrlDev + "/albums")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
	albums := []Album{}
	err = json.Unmarshal(body, &albums)
	if (err != nil) {
		fmt.Println(err)
	}
	return albums
}

func keepLines(s string, n int) string {
	result := strings.Join(strings.Split(s, "\n")[:n], "\n")
	return strings.Replace(result, "\r", "", -1)
}

func main() {
	fmt.Println(GetAlbums())
}