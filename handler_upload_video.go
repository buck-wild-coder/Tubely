package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	// TODO
	const maxMemory = 1 << 30
	http.MaxBytesReader(w, r.Body, maxMemory)

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Un-Authorized Access", err)
		return
	}

	videoFile, h, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not formfile", err)
		return
	}
	defer videoFile.Close()

	header := h.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "could not ParseMediaType", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "not a video file", fmt.Errorf("expected video/mp4, got %s", mediaType))
		return
	}

	tempFile, err := os.CreateTemp(cfg.assetsRoot, "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error in file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error in copying file", err)
		return
	}

	tempFile.Seek(0, io.SeekStart)

	ratio, err := getVideoAspectRatio(tempFile.Name())
	var orientation string
	if ratio == "16:9" {
		orientation = "landscape"
	} else {
		orientation = "portrait"
	}

	key := make([]byte, 32)
	rand.Read(key)
	name := base64.RawURLEncoding.EncodeToString(key)
	filePath := filepath.Join(cfg.assetsRoot, orientation, name)

	fastFilepath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "fast file", fmt.Errorf("Shit happens, %v", err.Error()))
		return
	}
	file, err := os.Create(filePath + ".mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error in file", err)
		return
	}
	defer file.Close()

	fastFile, err := os.Open(fastFilepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Fasr file opening error", err)
		return
	}
	defer fastFile.Close()
	defer os.Remove(fastFile.Name())

	_, err = io.Copy(file, fastFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error in copying file", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/%s", cfg.port, file.Name())
	dbVideo.VideoURL = &url
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error updating the video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
