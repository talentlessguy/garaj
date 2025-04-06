package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

var (
	addr      = flag.String("addr", ":8080", "TCP address to listen to")
	nodeAddr  = flag.String("nodeaddr", "localhost:5001", "TCP address of the IPFS node")
	maxBodyMB = flag.Int("max-body-mb", 32, "Maximum allowed CAR file size in megabytes")
)

func putCarFileToKubo(blob []byte, filename string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(blob)); err != nil {
		return fmt.Errorf("failed to write CAR data to form: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s/api/v0/dag/import?pin-roots=true&silent=false&stats=true&allow-big-block=false", *nodeAddr)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Kubo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kubo API returned error: %s - %s", resp.Status, string(body))
	}

	return nil
}

func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

const (
	CarContentType = "application/vnd.ipld.car"
)

func main() {
	flag.Parse()

	mux := http.NewServeMux()

	token, err := generateSecureToken(32)

	if err != nil {
		panic(err)
	}

	fmt.Printf("API key (stored in-memory): %s\n", token)

	maxBodyBytes := int64(*maxBodyMB) << 20

	mux.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "POST, OPTIONS")
		w.Header().Set("Accept", CarContentType)
		switch r.Method {

		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPost:

			if r.Header.Get("X-API-Key") != token {
				http.Error(w, `{"error": "Forbidden: Invalid API key"}`, http.StatusForbidden)
				return
			}
			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				http.Error(w, `{
					"error": "Bad Request",
					"message": "Content-Type header is required",
					"required_content_type": "`+CarContentType+`"
				}`, http.StatusBadRequest)
				return
			}
			if contentType != CarContentType {
				http.Error(w, `{
					"error": "Unsupported Media Type",
					"message": "Only CAR files are accepted",
					"received_content_type": "`+contentType+`",
					"required_content_type": "`+CarContentType+`"
				}`, http.StatusUnsupportedMediaType)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
			if err != nil {
				http.Error(w, `{"error": "Error reading request body"}`, http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			if len(body) == 0 {
				http.Error(w, `{"error": "Empty CAR file"}`, http.StatusBadRequest)
				return
			}
			if len(body) == int(maxBodyBytes) {
				http.Error(w, fmt.Sprintf(`{
					"error": "Payload Too Large",
					"message": "CAR file exceeds maximum size limit",
					"max_size_mb": %d
				}`, *maxBodyMB), http.StatusRequestEntityTooLarge)
				return
			}
			name := r.Header.Get("X-Filename")
			if name == "" {
				name = "file.car"
			}
			if err := putCarFileToKubo(body, name); err != nil {
				http.Error(w, `{"error": "Error processing CAR file"}`, http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "CAR file successfully processed"}`))

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			http.Error(w, `{"error": "Method Not Allowed"}`, http.StatusMethodNotAllowed)
		}

	},
	)
	fmt.Printf("Started a server on %s, file size limit %d MB\n", *addr, *maxBodyMB)
	http.ListenAndServe(*addr, mux)
}
