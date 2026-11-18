package relay

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// GeneratePasscode creates a random 6-character alphanumeric passphrase.
func GeneratePasscode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// UploadEncryptedBundle encrypts bundle data with a passcode and uploads to an ephemeral relay.
// Returns the sharing code (Passcode:RelayID) needed to retrieve and decrypt it.
func UploadEncryptedBundle(bundleBytes []byte, passcode string) (string, error) {
	encrypted, err := EncryptPayload(bundleBytes, passcode)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	// Upload to ephemeral relay (file.io auto-expiring after 1 download)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "relay-bundle.bin")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(encrypted)); err != nil {
		return "", err
	}
	_ = writer.WriteField("expires", "1d")
	_ = writer.WriteField("maxDownloads", "2")
	_ = writer.Close()

	req, err := http.NewRequest("POST", "https://file.io", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error during upload: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		Success bool   `json:"success"`
		Key     string `json:"key"`
		Link    string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("failed parsing relay response: %w", err)
	}

	if !res.Success || res.Key == "" {
		return "", fmt.Errorf("relay rejected upload")
	}

	// Sharing code: <KEY>:<PASSCODE>
	code := fmt.Sprintf("%s-%s", res.Key, passcode)
	return code, nil
}

// DownloadEncryptedBundle downloads and decrypts a bundle given its sharing code.
func DownloadEncryptedBundle(shareCode string) ([]byte, error) {
	parts := strings.Split(strings.TrimSpace(shareCode), "-")
	if len(parts) < 2 {
		return nil, fmt.Errorf("code de partage invalide (format attendu : ID-MOTDEPASSE)")
	}

	key := parts[0]
	passcode := parts[1]

	downloadURL := fmt.Sprintf("https://file.io/%s", key)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("network error during download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bundle introuvable ou déjà expiré (HTTP %d)", resp.StatusCode)
	}

	cipherText, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	plainText, err := DecryptPayload(cipherText, passcode)
	if err != nil {
		return nil, fmt.Errorf("échec de déchiffrement (mot de passe erroné ou données corrompues): %w", err)
	}

	return plainText, nil
}
