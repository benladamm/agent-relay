package relay

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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
// Returns the sharing code (Key-Passcode) needed to retrieve and decrypt it.
func UploadEncryptedBundle(bundleBytes []byte, passcode string) (string, error) {
	encrypted, err := EncryptPayload(bundleBytes, passcode)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files[]", "relay.bin")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(encrypted)); err != nil {
		return "", err
	}
	_ = writer.Close()

	req, err := http.NewRequest("POST", "https://uguu.se/upload.php", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "agent-relay/1.0 (curl-compatible)")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erreur réseau pendant le téléversement: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	var res struct {
		Success bool `json:"success"`
		Files   []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"files"`
	}

	if err := json.Unmarshal(respBytes, &res); err != nil {
		return "", fmt.Errorf("réponse inattendue du serveur de relais : %s", strings.TrimSpace(string(respBytes)))
	}

	if !res.Success || len(res.Files) == 0 || res.Files[0].URL == "" {
		return "", fmt.Errorf("le serveur de relais a refusé l'envoi")
	}

	rawURL := res.Files[0].URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("URL de relais invalide : %w", err)
	}

	// Host prefix e.g. "h" from "h.uguu.se"
	hostPrefix := strings.TrimSuffix(parsed.Host, ".uguu.se")
	filename := strings.TrimPrefix(parsed.Path, "/")

	// Format: <hostPrefix>_<filename>-<passcode>
	code := fmt.Sprintf("%s_%s-%s", hostPrefix, filename, passcode)
	return code, nil
}

// DownloadEncryptedBundle downloads and decrypts a bundle given its sharing code.
func DownloadEncryptedBundle(shareCode string) ([]byte, error) {
	shareCode = strings.TrimSpace(shareCode)
	idx := strings.LastIndex(shareCode, "-")
	if idx == -1 || idx == len(shareCode)-1 {
		return nil, fmt.Errorf("code de partage invalide (format attendu : ID-MOTDEPASSE)")
	}

	key := shareCode[:idx]
	passcode := shareCode[idx+1:]

	var candidateURLs []string
	if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
		candidateURLs = append(candidateURLs, key)
	} else if strings.Contains(key, "_") {
		parts := strings.SplitN(key, "_", 2)
		candidateURLs = append(candidateURLs, fmt.Sprintf("https://%s.uguu.se/%s", parts[0], parts[1]))
		candidateURLs = append(candidateURLs, fmt.Sprintf("https://h.uguu.se/%s", parts[1]))
		candidateURLs = append(candidateURLs, fmt.Sprintf("https://n.uguu.se/%s", parts[1]))
	} else {
		candidateURLs = append(candidateURLs, "https://h.uguu.se/"+key)
		candidateURLs = append(candidateURLs, "https://n.uguu.se/"+key)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	var lastErr error
	var cipherText []byte

	for _, u := range candidateURLs {
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "agent-relay/1.0 (curl-compatible)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && len(data) > 0 {
				cipherText = data
				lastErr = nil
				break
			}
		} else {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d sur %s", resp.StatusCode, u)
		}
	}

	if len(cipherText) == 0 {
		return nil, fmt.Errorf("impossible de récupérer le bundle (expiré ou indisponible): %v", lastErr)
	}

	plainText, err := DecryptPayload(cipherText, passcode)
	if err != nil {
		return nil, fmt.Errorf("échec de déchiffrement (mot de passe erroné ou données corrompues): %w", err)
	}

	return plainText, nil
}
