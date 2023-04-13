package main

import (
	"net/http"
	"time"
	"fmt"
)

const (
	BaseUrlDev = "http://localhost:7777/api"
)

type errorResponse struct {
	Code int		//'json:"code"'
	Message string	//'json:"message"'
}

type Client struct {
	BaseUrl string
	HTTPClient *http.Client
}

type Album struct {
	Album string	//'json:"albumId"'
	Name string		//'json:"name"'
	User string		//'json:"user"'
	PreviewId int	//'json:"previewId"'
}

func NewClient(api string) *Client {
	return &Client{
		BaseUrl: BaseUrlDev,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func (c *Client) GetAlbums() *http.Response {
	var resp, err = c.HTTPClient.Get(fmt.Sprintf("%s/albums", c.BaseUrl))
	if err != nil {
		fmt.Print(err)
	}
	return resp
}

func main() {
	var client = NewClient(BaseUrlDev)
	fmt.Print(client.GetAlbums().Body)
}