package sheets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type TokenFile struct {
	Token        string    `json:"token"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenURI     string    `json:"token_uri"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	Scopes       []string  `json:"scopes"`
	Expiry       time.Time `json:"expiry"`
}

type CredentialsFile struct {
	Installed *struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		TokenURI     string `json:"token_uri"`
	} `json:"installed"`
	Web *struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		TokenURI     string `json:"token_uri"`
	} `json:"web"`
}

type ValueRangeResponse struct {
	Range          string     `json:"range"`
	MajorDimension string     `json:"majorDimension"`
	Values         [][]string `json:"values"`
}

type Client struct {
	httpClient *http.Client
}

// NewClient initializes a Google Sheets API client using token.json and optional credentials.json
func NewClient(ctx context.Context, tokenPath string, credsPath string) (*Client, error) {
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file %s: %w", tokenPath, err)
	}

	var tf TokenFile
	if err := json.Unmarshal(tokenData, &tf); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	clientID := tf.ClientID
	clientSecret := tf.ClientSecret
	tokenURI := tf.TokenURI

	// If client_id or client_secret are not inside token.json, try credentials.json
	if (clientID == "" || clientSecret == "") && credsPath != "" {
		if credData, err := os.ReadFile(credsPath); err == nil {
			var cf CredentialsFile
			if json.Unmarshal(credData, &cf) == nil {
				if cf.Installed != nil {
					clientID = cf.Installed.ClientID
					clientSecret = cf.Installed.ClientSecret
					if tokenURI == "" {
						tokenURI = cf.Installed.TokenURI
					}
				} else if cf.Web != nil {
					clientID = cf.Web.ClientID
					clientSecret = cf.Web.ClientSecret
					if tokenURI == "" {
						tokenURI = cf.Web.TokenURI
					}
				}
			}
		}
	}

	endpoint := google.Endpoint
	if tokenURI != "" {
		endpoint.TokenURL = tokenURI
	}

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     endpoint,
		Scopes:       tf.Scopes,
	}

	accessToken := tf.Token
	if accessToken == "" {
		accessToken = tf.AccessToken
	}

	tok := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: tf.RefreshToken,
		TokenType:    "Bearer",
		Expiry:       tf.Expiry,
	}

	// TokenSource auto-refreshes expired access tokens using the refresh_token
	ts := conf.TokenSource(ctx, tok)
	httpClient := oauth2.NewClient(ctx, ts)
	httpClient.Timeout = 30 * time.Second

	return &Client{
		httpClient: httpClient,
	}, nil
}

// ReadSheet fetches all rows from a given sheet/worksheet name
func (c *Client) ReadSheet(spreadsheetID string, sheetName string) ([][]string, error) {
	escapedSheet := url.PathEscape(sheetName)
	apiURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s", spreadsheetID, escapedSheet)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call sheets API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sheets API error (status %d): %s", resp.StatusCode, string(body))
	}

	var res ValueRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode sheets response: %w", err)
	}

	return res.Values, nil
}

// UpdateSheet writes rows to a specific range in a spreadsheet
func (c *Client) UpdateSheet(spreadsheetID string, rangeName string, values [][]string) error {
	escapedRange := url.PathEscape(rangeName)
	apiURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s?valueInputOption=USER_ENTERED", spreadsheetID, escapedRange)

	payload := ValueRangeResponse{
		Range:          rangeName,
		MajorDimension: "ROWS",
		Values:         values,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal sheet values: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, apiURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call sheets update API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sheets update error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// ClearSheet clears all values from a sheet/range
func (c *Client) ClearSheet(spreadsheetID string, rangeName string) error {
	escapedRange := url.PathEscape(rangeName)
	apiURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s:clear", spreadsheetID, escapedRange)

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBufferString("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call sheets clear API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sheets clear error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
