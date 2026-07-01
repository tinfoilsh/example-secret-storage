package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/tinfoilsh/tinfoil-go/verifier/client"
)

func main() {
	storageHost := os.Getenv("STORAGE_HOST")
	if storageHost == "" {
		log.Fatal("STORAGE_HOST is required (e.g. secret-storage.tinfoil.containers.tinfoil.dev)")
	}
	storageRepo := os.Getenv("STORAGE_REPO")
	if storageRepo == "" {
		log.Fatal("STORAGE_REPO is required (e.g. tinfoilsh/confidential-secret-storage)")
	}
	userID := os.Getenv("USER_ID")
	if userID == "" {
		userID = "demo-user"
	}

	// Attest the storage enclave before sending any data.
	fmt.Printf("Attesting storage enclave %s (%s)...\n", storageHost, storageRepo)
	sc := client.NewSecureClient(storageHost, storageRepo)
	httpClient, err := sc.HTTPClient()
	if err != nil {
		log.Fatalf("attestation failed: %v", err)
	}
	fmt.Println("  Attested.")

	baseURL := "https://" + storageHost

	// 1. Generate and register encryption key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		log.Fatalf("generating key: %v", err)
	}
	keyB64 := base64.StdEncoding.EncodeToString(key)

	fmt.Printf("Uploading key for user %q...\n", userID)
	if err := postJSON(httpClient, baseURL+"/upload_key", map[string]string{
		"user_id": userID,
		"key":     keyB64,
	}, nil); err != nil {
		log.Fatalf("/upload_key: %v", err)
	}
	fmt.Println("  Key registered.")

	// 2. Store a sample secret
	sampleData := []byte("This is sensitive training data from " + userID)
	fmt.Printf("Storing %d bytes of data...\n", len(sampleData))

	var storeResp struct {
		ItemID string `json:"item_id"`
	}
	if err := postJSON(httpClient, baseURL+"/store", map[string]string{
		"user_id": userID,
		"data":    base64.StdEncoding.EncodeToString(sampleData),
	}, &storeResp); err != nil {
		log.Fatalf("/store: %v", err)
	}
	fmt.Printf("  Stored as item %s\n", storeResp.ItemID)

	fmt.Println("\nDone. The consumer's sync loop will fetch keys within 60s.")
}

func postJSON(c *http.Client, url string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, respBody)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
