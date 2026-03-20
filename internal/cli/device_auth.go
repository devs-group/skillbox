package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// DeviceAuthResponse represents the response from the device authorization endpoint.
type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse represents a successful token exchange response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// tokenErrorResponse is used internally to decode error responses from the token endpoint.
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// StartDeviceAuth initiates a device authorization flow with the given Hydra
// public URL and client ID. It returns the device/user codes and verification URI.
func StartDeviceAuth(hydraPublicURL, clientID string) (*DeviceAuthResponse, error) {
	endpoint := strings.TrimRight(hydraPublicURL, "/") + "/oauth2/device/auth"

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", "openid offline_access")

	resp, err := http.PostForm(endpoint, form)
	if err != nil {
		return nil, fmt.Errorf("device auth request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read device auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device auth failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var dar DeviceAuthResponse
	if err := json.Unmarshal(body, &dar); err != nil {
		return nil, fmt.Errorf("decode device auth response: %w", err)
	}

	return &dar, nil
}

// PollForToken polls the token endpoint until the user completes authentication,
// the code expires, or access is denied. It respects the polling interval and
// backs off on slow_down responses.
func PollForToken(hydraPublicURL, clientID, deviceCode string, interval int) (*TokenResponse, error) {
	endpoint := strings.TrimRight(hydraPublicURL, "/") + "/oauth2/token"

	if interval < 1 {
		interval = 5
	}

	for {
		time.Sleep(time.Duration(interval) * time.Second)

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("client_id", clientID)
		form.Set("device_code", deviceCode)

		resp, err := http.PostForm(endpoint, form)
		if err != nil {
			return nil, fmt.Errorf("token request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck
		if err != nil {
			return nil, fmt.Errorf("read token response: %w", err)
		}

		// Successful token exchange.
		if resp.StatusCode == http.StatusOK {
			var tok TokenResponse
			if err := json.Unmarshal(body, &tok); err != nil {
				return nil, fmt.Errorf("decode token response: %w", err)
			}
			return &tok, nil
		}

		// Check for expected error codes.
		var errResp tokenErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("unexpected response (HTTP %d): %s", resp.StatusCode, string(body))
		}

		switch errResp.Error {
		case "authorization_pending":
			// User hasn't completed login yet, keep polling.
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			return nil, fmt.Errorf("device code expired — please run login again")
		case "access_denied":
			return nil, fmt.Errorf("access denied by user")
		default:
			return nil, fmt.Errorf("token error: %s — %s", errResp.Error, errResp.ErrorDescription)
		}
	}
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	default:
		return fmt.Errorf("unsupported platform %s — please open %s manually", runtime.GOOS, rawURL)
	}
	return cmd.Start()
}
