package main

import (
	"net/http"
	"time"
	"fmt"
)

const (
	BaseUrlDev = "http://localhost:7777/api"
)

type Client struct {
	BaseUrl string
	HTTPClient *http.Client
}

func NewClient(api string) *Client {
	return &Client{
		BaseUrl: BaseUrlDev,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

func main() {
	fmt.Print("Hi there")
}