package util

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidState = errors.New("oauth: invalid state")
	ErrTokenExchange = errors.New("oauth: token exchange failed")
)

// ProviderConfig holds OAuth provider endpoints and credentials.
type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
}

// Token represents OAuth tokens returned from the provider.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// StateStore manages OAuth state for CSRF protection.
type StateStore interface {
	Set(ctx context.Context, state string, ttl time.Duration) error
	Validate(ctx context.Context, state string) (bool, error)
}

// OAuthClient handles OAuth authorization code flow.
type OAuthClient struct {
	cfg        ProviderConfig
	stateStore StateStore
	httpClient *http.Client
}

// NewOAuthClient creates a new OAuth client.
func NewOAuthClient(cfg ProviderConfig, stateStore StateStore) *OAuthClient {
	return &OAuthClient{
		cfg:        cfg,
		stateStore: stateStore,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GenerateState creates a cryptographically secure state parameter.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// AuthURL builds the authorization URL for redirecting the user.
func (c *OAuthClient) AuthURL(ctx context.Context, state string) (string, error) {
	if err := c.stateStore.Set(ctx, state, 10*time.Minute); err != nil {
		return "", err
	}

	u, err := url.Parse(c.cfg.AuthURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", c.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("state", state)
	if len(c.cfg.Scopes) > 0 {
		q.Set("scope", strings.Join(c.cfg.Scopes, " "))
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// Exchange validates the state and exchanges the authorization code for tokens.
func (c *OAuthClient) Exchange(ctx context.Context, code, state string) (*Token, error) {
	valid, err := c.stateStore.Validate(ctx, state)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrInvalidState
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.cfg.RedirectURL)
	data.Set("client_id", c.cfg.ClientID)
	data.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s", ErrTokenExchange, string(body))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// Refresh exchanges a refresh token for a new access token.
func (c *OAuthClient) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", c.cfg.ClientID)
	data.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s", ErrTokenExchange, string(body))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}