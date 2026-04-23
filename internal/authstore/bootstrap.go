package authstore

import (
	"fmt"

	"palace-manager/internal/config"
)

// EnsureBootstrap creates the initial user database when empty.
// Prefer migrating Manager.Username / Manager.Password from legacy config when set.
func EnsureBootstrap(s *Store, cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Users) > 0 {
		return nil
	}

	username := cfg.Manager.Username
	if username == "" {
		username = "admin"
	}

	pass := cfg.Manager.Password
	mustChange := true
	if pass != "" {
		mustChange = (pass == "changeme")
	} else {
		pass = "changeme"
		mustChange = true
	}

	hash, err := HashPassword(pass)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	s.Users = []User{{
		Username:           username,
		PasswordBcrypt:     hash,
		Role:               RoleAdmin,
		MustChangePassword: mustChange,
	}}
	return s.saveUnlocked()
}
