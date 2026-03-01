package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// hmacSHA256 calculates HMAC-SHA256 hash
func hmacSHA256(secret, message string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateDownloadToken generates HMAC-signed download token
// Returns: token, timestamp
func (c *Server) GenerateDownloadToken(filePath string) (string, string) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := filePath + "|" + timestamp
	token := hmacSHA256(Config().DownloadSecretKey, message)
	return token, timestamp
}

// VerifyHmacToken verifies HMAC-signed download token
// Returns: valid, error
func (c *Server) VerifyHmacToken(r *http.Request) (bool, error) {
	token := r.FormValue("token")
	timestamp := r.FormValue("ts")

	if token == "" || timestamp == "" {
		return false, fmt.Errorf("missing token or timestamp")
	}

	// Parse timestamp
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false, fmt.Errorf("invalid timestamp")
	}

	// Check expiration
	expire := int64(Config().DownloadTokenExpire)
	if expire <= 0 {
		expire = 600 // Default 10 minutes
	}
	now := time.Now().Unix()
	if now-ts > expire || ts-now > expire {
		return false, fmt.Errorf("token expired")
	}

	// Verify HMAC signature
	filePath := r.URL.Path
	message := filePath + "|" + timestamp
	expectedToken := hmacSHA256(Config().DownloadSecretKey, message)

	if !hmac.Equal([]byte(token), []byte(expectedToken)) {
		return false, fmt.Errorf("invalid token")
	}

	return true, nil
}

// SignDownloadURL handles API request to generate signed download URL
// Usage: GET /api/v1/sign_download?path=/group1/default/file.pdf
func (c *Server) SignDownloadURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	filePath := r.FormValue("path")
	if filePath == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    1,
			"message": "path is required",
		})
		return
	}

	// Check if HMAC auth is enabled
	if !Config().EnableHmacAuth || Config().DownloadSecretKey == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    1,
			"message": "HMAC auth is not enabled",
		})
		return
	}

	token, timestamp := c.GenerateDownloadToken(filePath)
	signedURL := fmt.Sprintf("%s?token=%s&ts=%s", filePath, token, timestamp)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 0,
		"data": map[string]string{
			"url":       signedURL,
			"token":     token,
			"timestamp": timestamp,
			"expire":    strconv.Itoa(Config().DownloadTokenExpire),
		},
	})
}
