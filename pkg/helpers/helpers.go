package helpers

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"jurobot/pkg/client"
	"jurobot/pkg/client/modules/chat"
	"jurobot/pkg/client/modules/collisions"
	"jurobot/pkg/client/modules/inventory"
	"jurobot/pkg/client/modules/physics"
	"jurobot/pkg/client/modules/playerlist"
	"jurobot/pkg/client/modules/protocol"
	"jurobot/pkg/client/modules/self"
	"jurobot/pkg/client/modules/world"
)

// PrismAccounts represents the structure of Prism Launcher's accounts.json
type PrismAccounts struct {
	Accounts []PrismAccount `json:"accounts"`
}

type PrismAccount struct {
	Type    string `json:"type"` // "MSA" or "Offline"
	Profile struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"profile"`
	MSA struct {
		RefreshToken string `json:"refresh_token"`
		Token        string `json:"token"`
	} `json:"msa"`
	Utoken struct {
		Token string `json:"token"`
		Exp   int64  `json:"exp"`
		Extra struct {
			UHS string `json:"uhs"`
		} `json:"extra"`
	} `json:"utoken"`
	MSAClientID string `json:"msa-client-id"`
}

// Flags holds common CLI flags for example bots.
type Flags struct {
	Address                   string
	Username                  string
	RefreshToken              string
	AccessToken               string
	XBLToken                  string
	XBLUserHash               string
	Verbose                   bool
	Interactive               bool
	TreatTransferAsDisconnect bool
	MaxReconnectAttempts      int
	Timeout                   int // in seconds
}

// RegisterFlags registers the standard CLI flags on the default flag set.
//
//	// -s <string> (server-address:port, default: localhost:25565)
//	// -u <string> (username, default: "" - determine from auth)
//	// -token <string> (refresh token for authentication)
//	// -v <bool> (verbose logging, default: false)
//	// -i <bool> (interactive mode with chat input, default: false)
//	// -d <bool> (treat server transfer packet as disconnect and reconnect, e.g. minehut sending player to lobby, default: false)
//	// -reconnects <int> (max reconnect attempts, default: 5)
//	// -timeout <int> (exit after N seconds, default: 0 - never)
func RegisterFlags(f *Flags) {
	flag.StringVar(&f.Address, "s", "localhost:25565", "server address (host:port)")
	flag.StringVar(&f.Username, "u", "", "username (offline or online)")
	flag.StringVar(&f.RefreshToken, "token", "", "refresh token for authentication")
	flag.BoolVar(&f.Verbose, "v", false, "verbose logging")
	flag.BoolVar(&f.Interactive, "i", false, "enable interactive mode with chat input")
	flag.BoolVar(&f.TreatTransferAsDisconnect, "d", false, "treat server transfer as disconnect")
	flag.IntVar(&f.MaxReconnectAttempts, "reconnects", 5, "max reconnect attempts (-1 = infinite, 0 = none)")
	flag.IntVar(&f.Timeout, "timeout", 0, "exit after N seconds (0 = never)")
}

// NewClient creates a client from parsed flags with default modules (protocol, self, world, chat).
func NewClient(f Flags) *client.Client {
	clientID := os.Getenv("AZURE_CLIENT_ID")
	if clientID == "" {
		clientID = "c36a9fb6-4f2a-41ff-90bd-ae7cc92031eb"
	}
	redirectPort, _ := strconv.Atoi(os.Getenv("AZURE_REDIRECT_PORT"))

	refreshToken := f.RefreshToken
	if refreshToken == "" {
		refreshToken = os.Getenv("MC_REFRESH_TOKEN")
	}

	accessToken := f.AccessToken
	if accessToken == "" {
		accessToken = os.Getenv("MC_ACCESS_TOKEN")
	}
	xblToken := f.XBLToken
	if xblToken == "" {
		xblToken = os.Getenv("MC_XBL_TOKEN")
	}
	xblUserHash := f.XBLUserHash
	if xblUserHash == "" {
		xblUserHash = os.Getenv("MC_XBL_USER_HASH")
	}

	// Always try to auto-discover from Prism Launcher for XBL token
	prismPath := GetPrismAccountsPath()
	if prismPath != "" {
		token, at, xbl, uhs, cid, err := GetPrismToken(prismPath, f.Username)
		if err == nil {
			refreshToken = token
			accessToken = at
			xblToken = xbl
			xblUserHash = uhs
			if cid != "" {
				clientID = cid
			}
		}
	}

	c := client.New(f.Address, f.Username, true)
	c.Verbose = f.Verbose
	c.ClientID = clientID
	c.RedirectPort = redirectPort
	c.RefreshToken = refreshToken
	c.AccessToken = accessToken
	c.XBLToken = xblToken
	c.XBLUserHash = xblUserHash
	c.Interactive = f.Interactive
	c.MaxReconnectAttempts = f.MaxReconnectAttempts

	proto := protocol.New()
	proto.TreatTransferAsDisconnect = f.TreatTransferAsDisconnect
	c.Register(proto)
	c.Register(self.New())
	c.Register(world.New())
	c.Register(chat.New())
	c.Register(inventory.New())
	c.Register(playerlist.New())
	c.Register(collisions.New())
	c.Register(physics.New())

	return c
}

// GetPrismAccountsPath returns the path to Prism Launcher accounts.json if it exists.
func GetPrismAccountsPath() string {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".local/share/PrismLauncher/accounts.json"),
		filepath.Join(home, ".var/app/org.prismlauncher.PrismLauncher/data/PrismLauncher/accounts.json"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ListPrismAccounts returns all MSA (authenticated) accounts from Prism Launcher.
func ListPrismAccounts(path string) ([]PrismAccount, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var accounts PrismAccounts
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}

	var msaAccounts []PrismAccount
	for _, acc := range accounts.Accounts {
		if acc.Type == "MSA" && acc.MSA.RefreshToken != "" {
			msaAccounts = append(msaAccounts, acc)
		}
	}

	return msaAccounts, nil
}

// GetPrismToken attempts to extract tokens and client ID for a username from Prism Launcher.
func GetPrismToken(path, username string) (token, accessToken, xblToken, xblUserHash, clientID string, err error) {
	accounts, err := ListPrismAccounts(path)
	if err != nil {
		return "", "", "", "", "", err
	}

	if username == "" {
		if len(accounts) == 1 {
			return accounts[0].MSA.RefreshToken, accounts[0].MSA.Token, accounts[0].Utoken.Token, accounts[0].Utoken.Extra.UHS, accounts[0].MSAClientID, nil
		}
		return "", "", "", "", "", fmt.Errorf("no username provided and multiple or no accounts found in PrismLauncher")
	}

	for _, acc := range accounts {
		if acc.Profile.Name == username {
			return acc.MSA.RefreshToken, acc.MSA.Token, acc.Utoken.Token, acc.Utoken.Extra.UHS, acc.MSAClientID, nil
		}
	}

	return "", "", "", "", "", fmt.Errorf("user %s not found in PrismLauncher", username)
}


// Run connects and starts the client, logging errors.
func Run(c *client.Client) {
	if err := c.ConnectAndStart(context.Background()); err != nil {
		c.Logger.Println(err)
	}
}
