package authstore

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const DefaultPath = "/etc/palace-manager/users.json"

// bcryptCost balances security vs provisioning latency on slow hosts.
const bcryptCost = bcrypt.DefaultCost

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleTenant Role = "tenant"
)

type User struct {
	Username           string   `json:"username"`
	PasswordBcrypt     string   `json:"password_bcrypt"`
	Role               Role     `json:"role"`
	Palaces            []string `json:"palaces"` // tenant: palace names; admin: empty or ["*"] means all
	MustChangePassword bool     `json:"must_change_password"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	Users []User `json:"users"`
}

func Load(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked()
}

func (s *Store) saveUnlocked() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := "/etc/palace-manager"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Get returns a copy of the user record (including hash) for verification.
func (s *Store) Get(username string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.Username == username {
			return u, true
		}
	}
	return User{}, false
}

func (s *Store) Verify(username, plaintext string) (User, bool) {
	u, ok := s.Get(username)
	if !ok || u.PasswordBcrypt == "" {
		return User{}, false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordBcrypt), []byte(plaintext)); err != nil {
		return User{}, false
	}
	return u, true
}

func HashPassword(plaintext string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CanAccessPalace checks whether the user may see/control the named palace.
func CanAccessPalace(role Role, allowed []string, palaceName string) bool {
	if role == RoleAdmin {
		return true
	}
	for _, p := range allowed {
		if p == palaceName {
			return true
		}
	}
	return false
}

var (
	ErrNotFound      = errors.New("user not found")
	ErrLastAdmin     = errors.New("cannot remove the last admin user")
	ErrInvalidRole   = errors.New("invalid role")
	ErrTenantPalaces = errors.New("tenant must have at least one palace")
)

func (s *Store) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, len(s.Users))
	copy(out, s.Users)
	return out
}

func (s *Store) SetPassword(username, newPlaintext string) error {
	h, err := HashPassword(newPlaintext)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].Username == username {
			s.Users[i].PasswordBcrypt = h
			s.Users[i].MustChangePassword = false
			return s.saveUnlocked()
		}
	}
	return ErrNotFound
}

func (s *Store) SetMustChange(username string, v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].Username == username {
			s.Users[i].MustChangePassword = v
			return s.saveUnlocked()
		}
	}
	return ErrNotFound
}

func (s *Store) Create(u User, plaintext string) error {
	if u.Username == "" {
		return errors.New("username required")
	}
	if u.Role != RoleAdmin && u.Role != RoleTenant {
		return ErrInvalidRole
	}
	if u.Role == RoleTenant && len(u.Palaces) == 0 {
		return ErrTenantPalaces
	}
	h, err := HashPassword(plaintext)
	if err != nil {
		return err
	}
	u.PasswordBcrypt = h

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ex := range s.Users {
		if ex.Username == u.Username {
			return errors.New("user already exists")
		}
	}
	s.Users = append(s.Users, u)
	return s.saveUnlocked()
}

func (s *Store) Update(username string, role Role, palaces []string, newPasswordOptional *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Users {
		if s.Users[i].Username == username {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	if role != RoleAdmin && role != RoleTenant {
		return ErrInvalidRole
	}
	if role == RoleTenant && len(palaces) == 0 {
		return ErrTenantPalaces
	}
	s.Users[idx].Role = role
	s.Users[idx].Palaces = append([]string(nil), palaces...)
	if newPasswordOptional != nil && *newPasswordOptional != "" {
		h, err := HashPassword(*newPasswordOptional)
		if err != nil {
			return err
		}
		s.Users[idx].PasswordBcrypt = h
		s.Users[idx].MustChangePassword = false
	}
	return s.saveUnlocked()
}

func (s *Store) Delete(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	adminCount := 0
	for _, u := range s.Users {
		if u.Role == RoleAdmin {
			adminCount++
		}
	}
	var target User
	found := false
	for _, u := range s.Users {
		if u.Username == username {
			target = u
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	if target.Role == RoleAdmin && adminCount <= 1 {
		return ErrLastAdmin
	}
	filtered := s.Users[:0]
	for _, u := range s.Users {
		if u.Username != username {
			filtered = append(filtered, u)
		}
	}
	s.Users = filtered
	return s.saveUnlocked()
}

func (s *Store) CountAdmins() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, u := range s.Users {
		if u.Role == RoleAdmin {
			n++
		}
	}
	return n
}

// PrimaryAdminUsername returns the first admin account in store order.
// Bootstrap writes the initial admin first, so this remains the primary admin
// unless users.json is manually reordered.
func (s *Store) PrimaryAdminUsername() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.Users {
		if u.Role == RoleAdmin {
			return u.Username
		}
	}
	return ""
}

func (s *Store) IsPrimaryAdmin(username string) bool {
	if username == "" {
		return false
	}
	return s.PrimaryAdminUsername() == username
}
