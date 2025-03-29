package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
	"github.com/jorgebay/jsonnav"
)

func processVideoForStart(filepath string) (string, error) {
	newPath := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newPath)
	cmd.Run()
	return newPath, nil
}
func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-print_format", "json", "-show_streams", filePath)
	fmt.Println(cmd.String())
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Run()
	jsonData, err := jsonnav.Unmarshal(buffer.String())
	if err != nil {
		fmt.Println("Cannot decipher into json struct")
		return "Error in converting to json", nil
	}
	aspectRatio := jsonData.Get("streams").Array().At(0).Get("display_aspect_ratio")
	arr := strings.Split(aspectRatio.String(), ":")
	width, _ := strconv.Atoi(arr[0])
	height, _ := strconv.Atoi(arr[1])
	if width == 9 && height == 16 {
		return "portrait", nil
	} else if width == 16 && height == 9 {
		return "landscape", nil
	} else {
		return "other", nil
	}
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

	io.Copy(tempFile, file)
	ratio, _ := getVideoAspectRatio(tempFile.Name())
	tempFile.Seek(0, io.SeekStart)
	processedFile, _ := processVideoForStart(tempFile.Name())
	newFile, _ := os.Open(processedFile)
	tempFile.Close()
	os.Remove(tempFile.Name())
	objectInput := s3.PutObjectInput{}
	bucket := "tubely-2005"
	objectInput.Bucket = &bucket
	objectInput.Body = newFile
	slice := make([]byte, 32)
	rand.Read(slice)
	key := ratio + "/" + base64.RawURLEncoding.EncodeToString(slice) + "." + "mp4"
	objectInput.Key = &key
	objectInput.ContentType = &fileType
	cfg.s3Client.PutObject(context.TODO(), &objectInput)
	vidUrl := "https://tubely-2005.s3.ap-south-1.amazonaws.com/" + key
	video.VideoURL = &vidUrl
	cfg.db.UpdateVideo(video)
}
