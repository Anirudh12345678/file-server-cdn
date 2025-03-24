package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMem int64 = 10 << 20
	r.ParseMultipartForm(maxMem)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to retrieve thumbnail", err)
	}
	contType := header.Header.Get("Content-Type")
	fileExt := strings.Split(contType, "/")[1]
	slice := make([]byte, 32)
	rand.Read(slice)
	idString := base64.RawURLEncoding.EncodeToString(slice)
	path := filepath.Join(cfg.assetsRoot, idString+"."+fileExt)
	createdFile, err := os.Create(path)
	if err != nil {
		return
	}
	io.Copy(createdFile, file)
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error", err)
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
	}

	thumbnailUrl := "http://localhost:8091/assets/" + idString + "." + fileExt
	video.ThumbnailURL = &thumbnailUrl
	cfg.db.UpdateVideo(video)
	respondWithJSON(w, http.StatusOK, video)
}
