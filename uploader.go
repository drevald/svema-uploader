package main

import (
	"net/http"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"bytes"
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

func PostAlbum(album Album) {
	albumJson, _ := json.Marshal(album)
	reader := bytes.NewReader(albumJson)
	fmt.Print(albumJson)
	_,_ = http.Post(BaseUrlDev + "/albums", "application/json", reader)

}


func main() {
	//fmt.Println(GetAlbums())
	album := Album {
		AlbumId : 1,
		Name: "name",
		User: 0,
		PreviewId: 0,
	}
	PostAlbum(album)
}