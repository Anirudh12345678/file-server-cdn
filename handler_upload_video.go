package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

type videoDims struct {
	height string
	width  string
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "-json", "show_streams", filePath)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	byteArray, _ := cmd.Output()
	fmt.Println(byteArray)
	var dims videoDims
	json.Unmarshal(byteArray, &dims)
	width, _ := strconv.Atoi(dims.width)
	height, _ := strconv.Atoi(dims.height)
	var res string
	fmt.Println(dims.width + ":" + dims.height)
	if width == 16 && height == 9 {
		res = "landscape"
	} else if height == 16 && width == 9 {
		res = "portrait"
	} else {
		res = "other"
	}
	return res, nil
}
func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	uploadLimit := 1 << 30
	videoIdString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIdString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error", err)
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
	}

	r.ParseMultipartForm(int64(uploadLimit))
	file, headers, err := r.FormFile("video")
	if err != nil {
		fmt.Println("Cannot retrieve video file")
		return
	}
	defer file.Close()

	fileType, _, err := mime.ParseMediaType(headers.Header.Get("Content-Type"))

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	io.Copy(tempFile, file)
	ratio, _ := getVideoAspectRatio("tubely-upload.mp4")
	tempFile.Seek(0, io.SeekStart)
	objectInput := s3.PutObjectInput{}
	bucket := "tubely-2005"
	objectInput.Bucket = &bucket
	objectInput.Body = tempFile
	slice := make([]byte, 32)
	rand.Read(slice)
	key := ratio + base64.RawURLEncoding.EncodeToString(slice) + "." + "mp4"
	objectInput.Key = &key
	objectInput.ContentType = &fileType
	cfg.s3Client.PutObject(context.TODO(), &objectInput)
	vidUrl := "https://tubely-2005.s3.ap-south-1.amazonaws.com/" + ratio + key
	video.VideoURL = &vidUrl
	cfg.db.UpdateVideo(video)
}
