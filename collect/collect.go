package collect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/virtual-portrait/photoapi"
)

var (
	Intsertcount = 0
	Updatecount  = 0
	// AwsAccessKeyID AWS access key
	AwsAccessKeyID string = os.Getenv("AWS_ACCESS_KEY_ID")
	// AwsSecretAccessKey AWS Secret key
	AwsSecretAccessKey string = os.Getenv("AWS_SECRET_ACCESS_KEY")
	// S3Region AWS region
	S3Region string = os.Getenv("S3_REGION")
	// S3Bucket destination bucket
	S3Bucket string = os.Getenv("S3_BUCKET")
)

type MediaItemsTotal struct {
	ClientID           int `json:"clientID"`
	TotalPages         int `json:"totalPages"`
	TotalMediainAlbum  int `json:"totalMediainAlbum"`
	TotalPhotosinAlbum int `json:"totalPhotosinAlbum"`
}

type MediaItem struct {
	ID            string `json:"id"`
	UploadStatus  bool   `json:uploadstatus",omitempty"`
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

func (data *MediaItemsTotal) insert(collection *mongo.Collection) {

	var result MediaItemsTotal
	filter := bson.D{{"clientid", data.ClientID}}

	err := collection.FindOne(context.TODO(), filter).Decode(&result)

	if err != nil {
		// case document does not exists
		insertResult, err := collection.InsertOne(context.TODO(), data)

		if err != nil {
			log.Fatal(err)
		}
		Intsertcount++
		fmt.Println("Inserted a single document: ", insertResult.InsertedID)
		return
	}

	// case a record already exists then update with values.

	replaceResult, err := collection.ReplaceOne(context.TODO(), filter, data)

	if err != nil {
		log.Fatal(err)
	}
	Updatecount++
	fmt.Printf("Replaced %v Documents!\n", replaceResult.ModifiedCount)

	return
}

func (data *MediaItem) insert(collection *mongo.Collection, wg *sync.WaitGroup) {
	defer wg.Done()

	var result MediaItem

	filter := bson.D{{"id", data.ID}}
	err := collection.FindOne(context.TODO(), filter).Decode(&result)

	if err != nil {
		// case document does not exists
		err := uploadtoS3(data.BaseURL, data.Filename)

		if err != nil {
			fmt.Println("Error trying to upload file to S3: ", err)
			data.UploadStatus = false
		} else {
			data.UploadStatus = true
		}

		insertResult, err := collection.InsertOne(context.TODO(), data)
		if err != nil {
			log.Fatal(err)
		}
		Intsertcount++
		fmt.Println("Inserted a single document: ", insertResult.InsertedID)
		return
	}

	if result.UploadStatus == false {
		// Case document exists but image is not in S3
		err := uploadtoS3(data.BaseURL, data.Filename)

		if err != nil {
			fmt.Println("Error trying to upload file to S3: ", err)
			data.UploadStatus = false
		} else {
			data.UploadStatus = true
		}
		// Update document
		filter := bson.D{{"id", data.ID}}
		r, err := collection.ReplaceOne(context.TODO(), filter, data)
		if err != nil {
			log.Fatal(err)
		}
		Updatecount++
		fmt.Printf("Updated %v Documents\n", r.ModifiedCount)
		uploadtoS3(data.BaseURL, data.Filename)
	}

	return

}

func connect(mongoConn string) *mongo.Client {
	clientOptions := options.Client().ApplyURI(mongoConn)

	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")

	return client

}

func closeConnect(client *mongo.Client) {

	err := client.Disconnect(context.TODO())

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connection to MongoDB closed.")

}

// AddFileToS3 upload files to the bucket in AWS S3
func AddFileToS3(s *session.Session, filename string) error {

	fmt.Println("Uploading file to S3")

	// Open the file for use
	file, err := os.Open("photos/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file size and read the file content into a buffer
	fileInfo, _ := file.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	// Config settings: this is where you choose the bucket, filename, content-type etc.
	// of the file you're uploading.
	_, err = s3.New(s).PutObject(&s3.PutObjectInput{
		Bucket:               aws.String(S3Bucket),
		Key:                  aws.String(filename),
		ACL:                  aws.String("private"),
		Body:                 bytes.NewReader(buffer),
		ContentLength:        aws.Int64(size),
		ContentType:          aws.String(http.DetectContentType(buffer)),
		ContentDisposition:   aws.String("attachment"),
		ServerSideEncryption: aws.String("AES256"),
	})

	if err != nil {
		return err
	}

	fmt.Println("File was uploaded")
	return err
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadfile(baseurl, filename string) error {

	// Get the media as data
	resp, err := http.Get(baseurl)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Create the file
	out, err := os.Create("photos/" + filename)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func removeFile(filename string) error {

	var err = os.Remove("photos/" + filename)
	if err != nil {
		return err
	}
	fmt.Println("File Deleted")
	return nil
}

// UploadtoS3 Connect to AWS and prepare for upload files
func uploadtoS3(baseurl, filename string) error {

	downloadfile(baseurl, filename)

	fmt.Println("Connecting to AWS")

	s, err := session.NewSession(&aws.Config{
		Region: aws.String(S3Region),
		Credentials: credentials.NewStaticCredentials(
			AwsAccessKeyID,
			AwsSecretAccessKey,
			"",
		),
	})
	if err != nil {
		return err
	}

	err = AddFileToS3(s, filename)
	if err != nil {
		return err
	}
	err = removeFile(filename)

	if err != nil {
		return err
	}
	return nil
}

// Save metadata to Mongo Database
func InsertAndSave(mediaItems []photoapi.MediaItems, MyClientID int, mongoConn string) {

	totalPhotos := 0
	totalMedia := 0
	var (
		collection = &mongo.Collection{}
		wg         sync.WaitGroup
	)

	client := connect(mongoConn)
	albumID := "album_" + strconv.Itoa(MyClientID)
	collection = client.Database(albumID).Collection("media_items")

	for _, i := range mediaItems {
		fmt.Println("Processing " + strconv.Itoa(len(i)) + " files")
		for _, v := range i {
			if v.MimeType != "video/mp4" {
				data := &MediaItem{
					ID:            v.ID,
					ProductURL:    v.ProductURL,
					BaseURL:       v.BaseURL,
					MimeType:      v.MimeType,
					MediaMetadata: v.MediaMetadata,
					Filename:      v.Filename,
				}
				wg.Add(1)
				go data.insert(collection, &wg)
				totalPhotos++
			}
			totalMedia++
		}
		wg.Wait()
	}

	data := &MediaItemsTotal{
		ClientID:           MyClientID,
		TotalPages:         len(mediaItems),
		TotalMediainAlbum:  totalMedia,
		TotalPhotosinAlbum: totalPhotos,
	}
	clientID := "client_" + strconv.Itoa(MyClientID)
	collection = client.Database("clients").Collection(clientID)

	data.insert(collection)

	closeConnect(client)

	fmt.Println("Total Pages:", len(mediaItems))
	fmt.Println("Total Media in Album:", totalMedia)
	fmt.Println("Total Photos in Album:", totalPhotos)
	fmt.Println("Total inserts:", Intsertcount)
	fmt.Println("Total Updates:", Updatecount)

}
