package photoapi

import (
	"errors"
	"fmt"
	"time"

	"github.com/parnurzeal/gorequest"
	"github.com/virtual-portrait/auth"
)

type Items struct {
	MediaItems    MediaItems `json:"mediaItems"`
	NextPageToken string     `json:"nextPageToken"`
}

type MediaItems []struct {
	ID            string `json:"id"`
	ProductURL    string `json:"productUrl"`
	BaseURL       string `json:"baseUrl"`
	MimeType      string `json:"mimeType"`
	MediaMetadata struct {
		CreationTime time.Time `json:"creationTime"`
		Width        string    `json:"width"`
		Height       string    `json:"height"`
		Photo        struct {
		} `json:"photo"`
	} `json:"mediaMetadata"`
	Filename string `json:"filename"`
}

type AlbumListParams struct {
	AlbumId   string `json:"albumId"`
	PageSize  string `json:"pageSize"`
	PageToken string `json:"pageToken"`
}

func (params *AlbumListParams) pageRequest(URL, tokenfile string) (MediaItems, string, error) {

	tok, _ := auth.TokenFromFile(tokenfile)

	request := gorequest.New().Timeout(60 * time.Second)
	resp := &Items{}

	httpResp, _, err := request.Post(URL).
		Set("Authorization", "Bearer "+tok.AccessToken).
		Send(params).
		EndStruct(resp)

	if err != nil {
		fmt.Println(err)
		return nil, "", errors.New("Error in API call to google API")
	}

	if httpResp.StatusCode != 200 {
		fmt.Println("Response code:", httpResp.StatusCode)
		fmt.Println(httpResp.Body)
		return nil, "", errors.New("Not able to pull report")

	}

	return resp.MediaItems, resp.NextPageToken, nil

}

// AlbumList get media (by pages) from given AlbumId
func AlbumList(URL, AlbumId, PageSize, tokenfile string) ([]MediaItems, error) {

	start := true
	counter := 1

	params := &AlbumListParams{
		PageSize:  PageSize,
		AlbumId:   AlbumId,
		PageToken: "",
	}

	var List []MediaItems

	fmt.Println("Get media by pages")
	for (start == true) || (params.PageToken != "") {

		start = false

		items, nextPageToken, err := params.pageRequest(URL, tokenfile)

		if err != nil {
			return nil, errors.New("Error in API call to google API")
		}
		List = append(List, items)

		params = &AlbumListParams{
			PageSize:  PageSize,
			AlbumId:   AlbumId,
			PageToken: nextPageToken,
		}
		fmt.Println("Pulled page:", counter)
		counter++

	}

	return List, nil

}
