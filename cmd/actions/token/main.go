package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	var brokerURL string

	flag.StringVar(&brokerURL, "broker-url", "", "URL of the p2-penny-pusher token broker")
	flag.Parse()

	if brokerURL == "" {
		log.Fatal("--broker-url is required")
	}

	// Get OIDC token from GitHub Actions environment
	oidcToken, err := getOIDCToken()
	if err != nil {
		log.Fatalf("get OIDC token: %v", err)
	}

	// Exchange OIDC token for installation token
	installToken, err := exchangeToken(brokerURL, oidcToken)
	if err != nil {
		log.Fatalf("exchange token: %v", err)
	}

	// Write to GITHUB_OUTPUT
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		log.Fatal("GITHUB_OUTPUT is not set")
	}

	file, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		log.Fatalf("open GITHUB_OUTPUT: %v", err)
	}
	defer file.Close()

	// Mask the token in logs
	fmt.Printf("::add-mask::%s\n", installToken)

	if _, err := fmt.Fprintf(file, "token=%s\n", installToken); err != nil {
		log.Fatalf("write output: %v", err)
	}

	fmt.Println("Successfully obtained installation token")
}

func getOIDCToken() (string, error) {
	requestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	requestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")

	if requestURL == "" || requestToken == "" {
		return "", fmt.Errorf("ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN must be set (ensure id-token: write permission)")
	}

	// Add audience parameter
	url := requestURL + "&audience=p2-penny-pusher"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "bearer "+requestToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OIDC request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode OIDC response: %w", err)
	}

	if result.Value == "" {
		return "", fmt.Errorf("OIDC response missing value")
	}

	return result.Value, nil
}

func exchangeToken(brokerURL, oidcToken string) (string, error) {
	// Normalize broker URL - remove trailing /token if present
	brokerURL = strings.TrimSuffix(brokerURL, "/token")
	brokerURL = strings.TrimSuffix(brokerURL, "/")

	url := brokerURL + "/token"

	payload := map[string]string{"token": oidcToken}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("broker returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode broker response: %w", err)
	}

	if result.Token == "" || result.Token == "null" {
		return "", fmt.Errorf("broker response missing token: %s", string(respBody))
	}

	return result.Token, nil
}
