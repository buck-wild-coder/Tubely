package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerVideoMetaCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		database.CreateVideoParams
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

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	params.UserID = userID

	video, err := cfg.db.CreateVideo(params.CreateVideoParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)
}

func (cfg *apiConfig) handlerVideoMetaDelete(w http.ResponseWriter, r *http.Request) {
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusForbidden, "You can't delete this video", err)
		return
	}

	err = cfg.db.DeleteVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete video", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerVideoGet(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func (cfg *apiConfig) handlerVideosRetrieve(w http.ResponseWriter, r *http.Request) {
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

	videos, err := cfg.db.GetVideos(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve videos", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videos)
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	type streams struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	type Ffprobe struct {
		Streams []streams `json:"streams"`
	}
	var ffprobe Ffprobe
	json.Unmarshal(stdout.Bytes(), &ffprobe)
	height := ffprobe.Streams[0].Height
	width := ffprobe.Streams[0].Width
	GCD := func(a, b int) int {
		for b != 0 {
			temp := b
			b = a % b
			a = temp
		}
		return a
	}
	gcd := GCD(width, height)
	if gcd == 0 {
		return "", fmt.Errorf("Cannot divide by 0")
	}
	ratio := fmt.Sprintf("%d:%d", width/gcd, height/gcd)
	return ratio, nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputFilePath, nil
}
