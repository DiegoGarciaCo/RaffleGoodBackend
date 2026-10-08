package handlers

// Presigned-upload endpoint. The app asks for a signed PUT URL, uploads the
// image straight to S3, then stores the returned public_url on the raffle
// (image_urls[]) or the org (image) via the existing create/update handlers.

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/diegoGarciaCo/raffles/internal/auth"
	"github.com/google/uuid"
)

// allowedImageTypes maps an accepted image Content-Type to its file extension.
var allowedImageTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/jpg":  "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/heic": "heic",
	"image/heif": "heif",
}

type presignUploadRequest struct {
	ContentType string `json:"content_type"` // e.g. "image/jpeg"
	Kind        string `json:"kind"`         // "raffle" | "org-logo"
}

type presignUploadResponse struct {
	UploadURL string `json:"upload_url"` // PUT the raw bytes here
	PublicURL string `json:"public_url"` // store this on the raffle/org
	Key       string `json:"key"`
	ExpiresIn int    `json:"expires_in"` // seconds the upload_url is valid
}

// HandlePresignUpload issues a short-lived presigned PUT URL for a single image.
// The app calls this once per photo, uploads each, and collects the public_urls.
//
//	POST /uploads/presign   (auth required)
func (cfg *apiCfg) HandlePresignUpload(w http.ResponseWriter, r *http.Request) {
	user := auth.MustUser(r.Context())

	var req presignUploadRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ext, ok := allowedImageTypes[strings.ToLower(strings.TrimSpace(req.ContentType))]
	if !ok {
		respondWithError(w, http.StatusBadRequest, "unsupported image type (use JPEG, PNG, WebP, or HEIC)")
		return
	}

	// Namespace by purpose (so bucket policies / lifecycle rules can target
	// each) and by a random UUID so uploads never collide.
	var prefix string
	switch req.Kind {
	case "org-logo":
		prefix = fmt.Sprintf("uploads/org-logos/%s", nonprofitID(user))
	default: // "raffle" and anything unspecified
		prefix = "uploads/raffles"
	}
	key := fmt.Sprintf("%s/%s.%s", prefix, uuid.NewString(), ext)

	const ttl = 5 * time.Minute
	uploadURL, err := cfg.Storage.PresignPut(r.Context(), key, req.ContentType, ttl)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not create upload URL")
		return
	}

	respondWithJSON(w, http.StatusOK, presignUploadResponse{
		UploadURL: uploadURL,
		PublicURL: cfg.Storage.PublicURL(key),
		Key:       key,
		ExpiresIn: int(ttl.Seconds()),
	})
}
