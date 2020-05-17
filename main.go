package main

import (
	"fmt"
	"os"
	"time"

	"github.com/virtual-portrait/auth"
	"github.com/virtual-portrait/collect"
	"github.com/virtual-portrait/photoapi"
)

var (
	URL      = "https://photoslibrary.googleapis.com/v1/mediaItems:search"
	AlbumId  = os.Getenv("GOOGLE_ALBUM_ID")
	PageSize = "100" // Maximun PageSize

	TokenFile   = "auth/token.json"
	Credentials = "auth/credentials.json"
	Scope       = "https://www.googleapis.com/auth/photoslibrary.readonly"
	MyClientID  = 1
	MongoConn   = os.Getenv("DATABASE_CONNECTION")
)

func main() {

	start := time.Now()

	auth.GetClient(TokenFile, Credentials, Scope)

	mediaItems, err := photoapi.AlbumList(URL, AlbumId, PageSize, TokenFile)

	if err != nil {
		panic("Couldn't get list of photos")
	}

	collect.InsertAndSave(mediaItems, MyClientID, MongoConn)

	duration := time.Since(start)
	fmt.Printf("%f seconds\n", duration.Seconds())
}
