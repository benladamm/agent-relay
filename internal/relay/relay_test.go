package relay_test

import (
	"bytes"
	"testing"

	"github.com/benladamm/agent-relay/internal/relay"
)

func TestRelayUploadDownload(t *testing.T) {
	sampleData := []byte("Hello from agent-relay! This is a test payload for encrypted session relay.")
	passcode := relay.GeneratePasscode()

	t.Logf("Generated passcode: %s", passcode)

	code, err := relay.UploadEncryptedBundle(sampleData, passcode)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	t.Logf("Received share code: %s", code)

	decrypted, err := relay.DownloadEncryptedBundle(code)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if !bytes.Equal(sampleData, decrypted) {
		t.Fatalf("Payload mismatch: got %s, want %s", string(decrypted), string(sampleData))
	}

	t.Logf("Success! Decrypted payload matches exactly: %s", string(decrypted))
}
