package roles

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type Role struct {
	Commands []string `json:"commands"`
}

type Data struct {
	Roles  map[string]Role `json:"roles"`
	Users  map[string][]string `json:"users"` // username -> list of roles
	Banned []string        `json:"banned"`
}

type Store struct {
	data     *Data
	mu       sync.RWMutex
	repo     string
	cacheDir string
}

func NewStore(repo string) *Store {
	cacheDir, _ := os.UserCacheDir()
	cacheDir = filepath.Join(cacheDir, "jurobot")
	os.MkdirAll(cacheDir, 0755)
	return &Store{repo: repo, cacheDir: cacheDir}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try loading via gh CLI (works with private repos)
	cmd := exec.Command("gh", "api", fmt.Sprintf("repos/%s/contents/roles.json", s.repo), "--jq", ".content")
	if out, err := cmd.Output(); err == nil {
		decoded, err := base64.StdEncoding.DecodeString(string(out))
		if err == nil {
			var d Data
			if err := json.Unmarshal(decoded, &d); err == nil {
				s.data = &d
				if raw, err := json.MarshalIndent(d, "", "  "); err == nil {
					os.WriteFile(filepath.Join(s.cacheDir, "roles.json"), raw, 0644)
				}
				return nil
			}
		}
	}

	cachePath := filepath.Join(s.cacheDir, "roles.json")
	raw, err := os.ReadFile(cachePath)
	if err != nil {
		s.data = &Data{
			Roles: map[string]Role{
				"owner":  {Commands: []string{"pull", "role", "ban", "inv", "list", "help"}},
				"pull":   {Commands: []string{"pull", "inv", "list", "help"}},
				"member": {Commands: []string{"inv", "list", "help"}},
			},
			Users:  map[string][]string{},
			Banned: []string{},
		}
		return nil
	}
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	s.data = &d
	return nil
}

func (s *Store) IsBanned(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return false
	}
	for _, b := range s.data.Banned {
		if b == username {
			return true
		}
	}
	return false
}

func (s *Store) IsOwner(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return false
	}
	for _, r := range s.data.Users[username] {
		if r == "owner" {
			return true
		}
	}
	return false
}

func (s *Store) HasRole(username, role string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return false
	}
	for _, r := range s.data.Users[username] {
		if r == role {
			return true
		}
	}
	return false
}

func (s *Store) CanUseCommand(username, command string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return false
	}
	if s.isBannedLocked(username) {
		return false
	}
	userRoles := s.data.Users[username]
	if len(userRoles) == 0 {
		userRoles = []string{"member"}
	}
	for _, roleName := range userRoles {
		role, ok := s.data.Roles[roleName]
		if !ok {
			continue
		}
		for _, cmd := range role.Commands {
			if cmd == command {
				return true
			}
		}
	}
	return false
}

// RoleCanUseCommand checks if a specific role grants access to a command.
func (s *Store) RoleCanUseCommand(roleName, command string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return false
	}
	role, ok := s.data.Roles[roleName]
	if !ok {
		return false
	}
	for _, cmd := range role.Commands {
		if cmd == command {
			return true
		}
	}
	return false
}

func (s *Store) isBannedLocked(username string) bool {
	for _, b := range s.data.Banned {
		if b == username {
			return true
		}
	}
	return false
}

func (s *Store) GetUserRoles(username string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return nil
	}
	return s.data.Users[username]
}

func (s *Store) AddRole(username, roleName string) error {
	s.mu.Lock()
	if s.data == nil {
		s.mu.Unlock()
		return fmt.Errorf("roles not loaded")
	}
	if _, ok := s.data.Roles[roleName]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("role %q does not exist", roleName)
	}
	for _, r := range s.data.Users[username] {
		if r == roleName {
			s.mu.Unlock()
			return fmt.Errorf("%s already has role %s", username, roleName)
		}
	}
	s.data.Users[username] = append(s.data.Users[username], roleName)
	s.mu.Unlock()
	go s.save()
	return nil
}

func (s *Store) RemoveRole(username, roleName string) error {
	s.mu.Lock()
	if s.data == nil {
		s.mu.Unlock()
		return fmt.Errorf("roles not loaded")
	}
	roles := s.data.Users[username]
	found := false
	for i, r := range roles {
		if r == roleName {
			s.data.Users[username] = append(roles[:i], roles[i+1:]...)
			if len(s.data.Users[username]) == 0 {
				delete(s.data.Users, username)
			}
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found {
		return fmt.Errorf("%s doesn't have role %s", username, roleName)
	}
	go s.save()
	return nil
}

func (s *Store) Ban(username string) error {
	s.mu.Lock()
	if s.data == nil {
		s.mu.Unlock()
		return fmt.Errorf("roles not loaded")
	}
	for _, b := range s.data.Banned {
		if b == username {
			s.mu.Unlock()
			return fmt.Errorf("%s is already banned", username)
		}
	}
	s.data.Banned = append(s.data.Banned, username)
	s.mu.Unlock()
	go s.save()
	return nil
}

func (s *Store) Unban(username string) error {
	s.mu.Lock()
	if s.data == nil {
		s.mu.Unlock()
		return fmt.Errorf("roles not loaded")
	}
	found := false
	for i, b := range s.data.Banned {
		if b == username {
			s.data.Banned = append(s.data.Banned[:i], s.data.Banned[i+1:]...)
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found {
		return fmt.Errorf("%s is not banned", username)
	}
	go s.save()
	return nil
}

func (s *Store) ListRoles() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return nil
	}
	var names []string
	for name := range s.data.Roles {
		names = append(names, name)
	}
	return names
}

func (s *Store) ListUsers() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return nil
	}
	out := make(map[string][]string)
	for user, roles := range s.data.Users {
		out[user] = roles
	}
	return out
}

func (s *Store) Reload() error {
	return s.Load()
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	cachePath := filepath.Join(s.cacheDir, "roles.json")
	if err := os.WriteFile(cachePath, raw, 0644); err != nil {
		return err
	}

	// Get current file SHA
	shaOut, err := exec.Command("gh", "api", fmt.Sprintf("repos/%s/contents/roles.json", s.repo), "--jq", ".sha").Output()
	if err != nil {
		return fmt.Errorf("failed to get file SHA: %v", err)
	}
	sha := string(shaOut)

	// Update via GitHub API
	cmd := exec.Command("gh", "api", fmt.Sprintf("repos/%s/contents/roles.json", s.repo),
		"-X", "PUT",
		"-f", fmt.Sprintf("message=%s", "update roles.json"),
		"-f", fmt.Sprintf("content=%s", base64.StdEncoding.EncodeToString(raw)),
		"-f", fmt.Sprintf("sha=%s", sha),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("API update failed: %v — %s", err, string(out))
	}

	return nil
}
