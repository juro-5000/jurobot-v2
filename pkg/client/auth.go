package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"jurobot/pkg/chat"
	auth "github.com/go-mclib/protocol/auth"
	ns "github.com/go-mclib/protocol/java_protocol/net_structures"
	session_server "github.com/go-mclib/protocol/java_protocol/session_server"
)

// ── Token type ──

type azureTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

// ── Xbox → Minecraft types ──

type xblRequest struct {
	Properties struct {
		AuthMethod string `json:"AuthMethod"`
		SiteName   string `json:"SiteName"`
		RpsTicket  string `json:"RpsTicket"`
	} `json:"Properties"`
	RelyingParty string `json:"RelyingParty"`
	TokenType    string `json:"TokenType"`
}

type xblResponse struct {
	Token         string `json:"Token"`
	DisplayClaims struct {
		XUI []struct {
			UHS string `json:"uhs"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

type xstsRequest struct {
	Properties struct {
		SandboxID  string   `json:"SandboxId"`
		UserTokens []string `json:"UserTokens"`
	} `json:"Properties"`
	RelyingParty string `json:"RelyingParty"`
	TokenType    string `json:"TokenType"`
}

type xstsResponse struct {
	Token         string `json:"Token"`
	DisplayClaims struct {
		XUI []struct {
			UHS string `json:"uhs"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

type mcLoginRequest struct {
	IdentityToken string `json:"identityToken"`
}

type mcLoginResponse struct {
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type entitlementsResponse struct {
	Items []struct {
		Name string `json:"name"`
	} `json:"items"`
}

type mcProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ── Xbox → Minecraft chain ──

func xblAuthenticate(ctx context.Context, httpClient *http.Client, msAccessToken string) (*xblResponse, error) {
	body := xblRequest{
		RelyingParty: "http://auth.xboxlive.com",
		TokenType:    "JWT",
	}
	body.Properties.AuthMethod = "RPS"
	body.Properties.SiteName = "user.auth.xboxlive.com"
	body.Properties.RpsTicket = "d=" + msAccessToken
	buf, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://user.auth.xboxlive.com/user/authenticate", strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("xbl authenticate: %s: %s", res.Status, string(data))
	}

	var out xblResponse
	json.NewDecoder(res.Body).Decode(&out)
	return &out, nil
}

func xstsAuthorize(ctx context.Context, httpClient *http.Client, xblToken string) (*xstsResponse, error) {
	body := xstsRequest{
		RelyingParty: "rp://api.minecraftservices.com/",
		TokenType:    "JWT",
	}
	body.Properties.SandboxID = "RETAIL"
	body.Properties.UserTokens = []string{xblToken}
	buf, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://xsts.auth.xboxlive.com/xsts/authorize", strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("xsts authorize: %s: %s", res.Status, string(data))
	}

	var out xstsResponse
	json.NewDecoder(res.Body).Decode(&out)
	return &out, nil
}

func mcLoginWithXbox(ctx context.Context, httpClient *http.Client, userHash, xstsToken string) (*mcLoginResponse, error) {
	body := mcLoginRequest{IdentityToken: fmt.Sprintf("XBL3.0 x=%s;%s", userHash, xstsToken)}
	buf, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.minecraftservices.com/authentication/login_with_xbox", strings.NewReader(string(buf)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("mc login: %s: %s", res.Status, string(data))
	}

	var out mcLoginResponse
	json.NewDecoder(res.Body).Decode(&out)
	return &out, nil
}

func checkMcOwnership(ctx context.Context, httpClient *http.Client, token string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.minecraftservices.com/entitlements/mcstore", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return fmt.Errorf("entitlements: %s: %s", res.Status, string(data))
	}

	var out entitlementsResponse
	json.NewDecoder(res.Body).Decode(&out)
	if len(out.Items) == 0 {
		return fmt.Errorf("account does not own Minecraft")
	}
	return nil
}

func fetchMcProfile(ctx context.Context, httpClient *http.Client, token string) (*mcProfile, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.minecraftservices.com/minecraft/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no Minecraft profile")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("profile: %s: %s", res.Status, string(data))
	}

	var out mcProfile
	json.NewDecoder(res.Body).Decode(&out)
	return &out, nil
}

// ── Refresh token auth (Azure AD v2.0) ──

func refreshWithAzureAD(ctx context.Context, clientID, refreshToken string) (string, error) {
	if clientID == "" {
		clientID = "c36a9fb6-4f2a-41ff-90bd-ae7cc92031eb"
	}
	resp, err := http.PostForm("https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		url.Values{
			"client_id":     {clientID},
			"grant_type":    {"refresh_token"},
			"refresh_token": {refreshToken},
			"scope":         {"XboxLive.signin offline_access"},
		})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tr azureTokenResponse
	json.Unmarshal(body, &tr)
	if tr.AccessToken == "" {
		return "", fmt.Errorf("token refresh failed: %s", string(body))
	}
	return tr.AccessToken, nil
}

func refreshTokenAuth(ctx context.Context, clientID, refreshToken string) (auth.LoginData, error) {
	if clientID == "" {
		clientID = "c36a9fb6-4f2a-41ff-90bd-ae7cc92031eb"
	}

	accessToken, err := refreshWithAzureAD(ctx, clientID, refreshToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}

	xblRes, err := xblAuthenticate(ctx, httpClient, accessToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	xstsRes, err := xstsAuthorize(ctx, httpClient, xblRes.Token)
	if err != nil {
		return auth.LoginData{}, err
	}

	mcRes, err := mcLoginWithXbox(ctx, httpClient, xblRes.DisplayClaims.XUI[0].UHS, xstsRes.Token)
	if err != nil {
		return auth.LoginData{}, err
	}

	if err := checkMcOwnership(ctx, httpClient, mcRes.AccessToken); err != nil {
		return auth.LoginData{}, err
	}

	profile, err := fetchMcProfile(ctx, httpClient, mcRes.AccessToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	return auth.LoginData{
		AccessToken:  mcRes.AccessToken,
		RefreshToken: refreshToken,
		UUID:         profile.ID,
		Username:     profile.Name,
		ExpiresAt:    time.Now().Add(time.Duration(mcRes.ExpiresIn) * time.Second),
	}, nil
}

// ── initializeAuth ──

func (c *Client) initializeAuth(ctx context.Context) error {
	// Try XBL token first (e.g. from Prism Launcher's cached Xbox User Token)
	if c.XBLToken != "" {
		c.Logger.Println("Authenticating with Xbox User Token from Prism Launcher...")
		ld, err := xblTokenAuth(ctx, c.XBLToken, c.XBLUserHash)
		if err == nil {
			c.LoginData = ld
			c.Username = ld.Username
			return c.finishAuth(ctx, ld)
		}
		c.Logger.Printf("XBL token auth failed (%v), trying refresh token...", err)
	}

	// Try direct access token (e.g. from Prism Launcher's cached MSA token)
	if c.AccessToken != "" {
		c.Logger.Println("Authenticating with Prism Launcher access token...")
		ld, err := accessTokenAuth(ctx, c.AccessToken)
		if err == nil {
			c.LoginData = ld
			c.Username = ld.Username
			return c.finishAuth(ctx, ld)
		}
		c.Logger.Printf("Access token auth failed (%v), trying refresh token...", err)
	}

	if c.RefreshToken == "" {
		return fmt.Errorf("no refresh token, access token, or XBL token available — please add a Microsoft account in Prism Launcher")
	}

	c.Logger.Println("Authenticating with Prism Launcher refresh token...")
	ld, err := refreshTokenAuth(ctx, c.ClientID, c.RefreshToken)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	c.LoginData = ld

	return c.finishAuth(ctx, ld)
}

func xblTokenAuth(ctx context.Context, xblToken, userHash string) (auth.LoginData, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}

	xstsRes, err := xstsAuthorize(ctx, httpClient, xblToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	mcRes, err := mcLoginWithXbox(ctx, httpClient, userHash, xstsRes.Token)
	if err != nil {
		return auth.LoginData{}, err
	}

	if err := checkMcOwnership(ctx, httpClient, mcRes.AccessToken); err != nil {
		return auth.LoginData{}, err
	}

	profile, err := fetchMcProfile(ctx, httpClient, mcRes.AccessToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	return auth.LoginData{
		AccessToken: mcRes.AccessToken,
		UUID:        profile.ID,
		Username:    profile.Name,
		ExpiresAt:   time.Now().Add(time.Duration(mcRes.ExpiresIn) * time.Second),
	}, nil
}

func accessTokenAuth(ctx context.Context, msAccessToken string) (auth.LoginData, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}

	xblRes, err := xblAuthenticate(ctx, httpClient, msAccessToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	xstsRes, err := xstsAuthorize(ctx, httpClient, xblRes.Token)
	if err != nil {
		return auth.LoginData{}, err
	}

	mcRes, err := mcLoginWithXbox(ctx, httpClient, xblRes.DisplayClaims.XUI[0].UHS, xstsRes.Token)
	if err != nil {
		return auth.LoginData{}, err
	}

	if err := checkMcOwnership(ctx, httpClient, mcRes.AccessToken); err != nil {
		return auth.LoginData{}, err
	}

	profile, err := fetchMcProfile(ctx, httpClient, mcRes.AccessToken)
	if err != nil {
		return auth.LoginData{}, err
	}

	return auth.LoginData{
		AccessToken: mcRes.AccessToken,
		UUID:        profile.ID,
		Username:    profile.Name,
		ExpiresAt:   time.Now().Add(time.Duration(mcRes.ExpiresIn) * time.Second),
	}, nil
}

func (c *Client) finishAuth(ctx context.Context, ld auth.LoginData) error {
	if c.Username != "" && c.Username != ld.Username {
		c.Logger.Printf("Warning: authenticated as '%s' but requested username was '%s' (credentials may have changed)", ld.Username, c.Username)
	}
	c.Username = ld.Username

	cert, err := auth.FetchMojangCertificate(ld.AccessToken)
	if err != nil {
		return fmt.Errorf("fetch certificate: %w", err)
	}

	c.ChatSigner = chat.NewChatSigner()
	c.ChatSigner.SetKeys(cert.PrivateKey, cert.PublicKey)

	playerUUID, err := ns.UUIDFromString(ld.UUID)
	if err != nil {
		return fmt.Errorf("parse player uuid: %w", err)
	}
	c.ChatSigner.PlayerUUID = playerUUID
	c.ChatSigner.AddPlayerPublicKey(playerUUID, cert.PublicKey)

	c.ChatSigner.X509PublicKey = cert.PublicKeyBytes

	mojangSig, err := base64.StdEncoding.DecodeString(cert.Certificate.PublicKeySignatureV2)
	if err != nil {
		return fmt.Errorf("decode mojang signature: %w", err)
	}
	c.ChatSigner.SessionKey = mojangSig
	if expiry, err := time.Parse(time.RFC3339Nano, cert.Certificate.ExpiresAt); err == nil {
		c.ChatSigner.KeyExpiry = expiry
	}

	c.SessionClient = session_server.NewSessionServerClient()
	return nil
}
