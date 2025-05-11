package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	// OAuth2CredentialsFile is the filename for the OAuth2 credentials.
	OAuth2CredentialsFile = "credentials.json"
)

var (
	// Scopes defines the scopes required for the Gmail API.
	Scopes = []string{gmail.GmailReadonlyScope}
)

// GetCredentials retrieves the OAuth2 credentials for the specified data directory.
// If credentials don't exist in the data directory, it runs the authorization flow.
func GetCredentials(dataDir string) (*oauth2.Token, error) {
	if _, err := os.Stat(OAuth2CredentialsFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials.json not found")
	}

	b, err := ioutil.ReadFile(OAuth2CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client credentials file: %v", err)
	}

	// Configure the OAuth2 config.
	config, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client credentials: %v", err)
	}

	credentialsFile := filepath.Join(dataDir, "credentials.json")
	tok, err := tokenFromFile(credentialsFile)
	if err != nil {
		// If no credentials are found, start the authorization flow.
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(credentialsFile, tok); err != nil {
			return nil, err
		}
	}

	return tok, nil
}

// GetService creates a Gmail API service using the provided OAuth2 token.
func GetService(tok *oauth2.Token) (*gmail.Service, error) {
	b, err := ioutil.ReadFile(OAuth2CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client credentials: %v", err)
	}

	client := config.Client(context.Background(), tok)
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create Gmail service: %v", err)
	}

	return srv, nil
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// getTokenFromWeb requests a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

// saveToken saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache OAuth token: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}