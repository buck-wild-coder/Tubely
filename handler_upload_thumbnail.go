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

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't ParseMultipartForm", err)
		return
	}

	imagefile, h, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't FormFile", err)
		return
	}
	defer imagefile.Close()

	contentType := h.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "ParseMediaType", err)
		return
	}
	if mediaType != "image/png" && mediaType != "image/jpeg" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Wrong file type", fmt.Errorf("Expected image type, got %s", mediaType))
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't Get video from db", err)
		return
	}
	if dbVideo.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "not authorized", nil)
		return
	}

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Can't use rand.Read()", err)
		return
	}
	name := base64.RawURLEncoding.EncodeToString(b)

	nmediaType := strings.Split(mediaType, "/")
	extension := nmediaType[1]
	filePath := filepath.Join(cfg.assetsRoot, name+"."+extension)
	file, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "File uploading error", err)
		return
	}
	defer file.Close()

	_, err = io.Copy(file, imagefile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "can't copy", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, name, extension)
	dbVideo.ThumbnailURL = &url
	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error updating the video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
