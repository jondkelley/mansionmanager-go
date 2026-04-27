package authstore

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const DefaultPath = "/etc/palace-manager/users.json"

// bcryptCost balances security vs provisioning latency on slow hosts.
const bcryptCost = bcrypt.DefaultCost

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleTenant     Role = "tenant"
	RoleSubaccount Role = "subaccount"
)

type User struct {
	Username           string              `json:"username"`
	PasswordBcrypt     string              `json:"password_bcrypt"`
	Role               Role                `json:"role"`
	Palaces            []string            `json:"palaces"` // tenant: palace names; admin: empty or ["*"] means all
	ParentTenant       string              `json:"parent_tenant,omitempty"` // subaccount: owning tenant username
	PalacePerms        map[string][]string `json:"palace_perms,omitempty"`  // subaccount: palace -> permission keys
	MustChangePassword bool                `json:"must_change_password"`
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

// CanAccessPalace checks whether the user may see the named palace in lists and GET metadata.
// For subaccounts, palaceName must appear in palacePerms with a non-empty permission list.
func CanAccessPalace(role Role, tenantPalaces []string, palacePerms map[string][]string, palaceName string) bool {
	switch role {
	case RoleAdmin:
		return true
	case RoleTenant:
		for _, p := range tenantPalaces {
			if p == palaceName {
				return true
			}
		}
		return false
	case RoleSubaccount:
		if palacePerms == nil {
			return false
		}
		list, ok := palacePerms[palaceName]
		return ok && len(list) > 0
	default:
		return false
	}
}

var (
	ErrNotFound            = errors.New("user not found")
	ErrLastAdmin           = errors.New("cannot remove the last admin user")
	ErrInvalidRole         = errors.New("invalid role")
	ErrTenantPalaces       = errors.New("tenant must have at least one palace")
	ErrSubaccountPalaces = errors.New("subaccount must have at least one palace with permissions")
	ErrSubaccountParent  = errors.New("subaccount parent must be an existing tenant")
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
	if u.Role == RoleSubaccount {
		return errors.New("use CreateSubaccount for subaccount users")
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
	if s.Users[idx].Role == RoleSubaccount {
		return errors.New("cannot update subaccount via Update")
	}
	if role == RoleSubaccount {
		return ErrInvalidRole
	}
	if role != RoleAdmin && role != RoleTenant {
		return ErrInvalidRole
	}
	if role == RoleTenant && len(palaces) == 0 {
		return ErrTenantPalaces
	}
	s.Users[idx].Role = role
	s.Users[idx].Palaces = append([]string(nil), palaces...)
	s.Users[idx].ParentTenant = ""
	s.Users[idx].PalacePerms = nil
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

// RenamePalaceInTenantBindings replaces oldName with newName in every tenant user's palace list
// and in every subaccount's palace_perms map keys.
func (s *Store) RenamePalaceInTenantBindings(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Users {
		if s.Users[i].Role == RoleTenant {
			for j := range s.Users[i].Palaces {
				if s.Users[i].Palaces[j] == oldName {
					s.Users[i].Palaces[j] = newName
				}
			}
		}
		if s.Users[i].Role == RoleSubaccount && s.Users[i].PalacePerms != nil {
			if perms, ok := s.Users[i].PalacePerms[oldName]; ok {
				delete(s.Users[i].PalacePerms, oldName)
				s.Users[i].PalacePerms[newName] = append([]string(nil), perms...)
			}
		}
	}
	return s.saveUnlocked()
}

// RemovePalaceAfterPermanentDelete drops palaceName from every tenant's palace list and from
// every subaccount's palace_perms. Tenants left with no palaces are removed together with
// their subaccounts. Subaccounts left with no delegated palaces after pruning are removed.
func (s *Store) RemovePalaceAfterPermanentDelete(palaceName string) error {
	palaceName = strings.TrimSpace(palaceName)
	if palaceName == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantPalacesAfter := make(map[string][]string)
	for _, u := range s.Users {
		if u.Role != RoleTenant {
			continue
		}
		fp := filterPalaceNameOut(u.Palaces, palaceName)
		if len(fp) > 0 {
			tenantPalacesAfter[u.Username] = fp
		}
	}

	allowed := func(palaces []string) map[string]struct{} {
		m := make(map[string]struct{}, len(palaces))
		for _, p := range palaces {
			if p != "" {
				m[p] = struct{}{}
			}
		}
		return m
	}

	var out []User
	for _, u := range s.Users {
		switch u.Role {
		case RoleAdmin:
			out = append(out, u)
		case RoleTenant:
			fp, ok := tenantPalacesAfter[u.Username]
			if !ok {
				continue
			}
			nu := u
			nu.Palaces = fp
			out = append(out, nu)
		case RoleSubaccount:
			parentPalaces, parentOK := tenantPalacesAfter[u.ParentTenant]
			if !parentOK {
				continue
			}
			parentSet := allowed(parentPalaces)
			pm := NormalizePalacePerms(u.PalacePerms)
			if len(pm) == 0 {
				continue
			}
			newMap := make(map[string][]string)
			for k, permList := range pm {
				if _, ok := parentSet[k]; !ok {
					continue
				}
				if len(permList) == 0 {
					continue
				}
				newMap[k] = append([]string(nil), permList...)
			}
			npm := NormalizePalacePerms(newMap)
			if len(npm) == 0 {
				continue
			}
			nu := u
			nu.PalacePerms = npm
			out = append(out, nu)
		default:
			out = append(out, u)
		}
	}
	s.Users = out
	return s.saveUnlocked()
}

func filterPalaceNameOut(palaces []string, name string) []string {
	out := make([]string, 0, len(palaces))
	for _, p := range palaces {
		if p != "" && p != name {
			out = append(out, p)
		}
	}
	return out
}

// PruneSubaccountPalacesForTenant removes palace keys from all subaccounts of tenantUsername
// that are not in allowedPalaces. Returns error if any subaccount would be left with no palaces.
func (s *Store) PruneSubaccountPalacesForTenant(tenantUsername string, allowedPalaces []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	allowed := make(map[string]struct{}, len(allowedPalaces))
	for _, p := range allowedPalaces {
		if p != "" {
			allowed[p] = struct{}{}
		}
	}
	for i := range s.Users {
		if s.Users[i].Role != RoleSubaccount || s.Users[i].ParentTenant != tenantUsername {
			continue
		}
		m := s.Users[i].PalacePerms
		if len(m) == 0 {
			continue
		}
		newMap := make(map[string][]string)
		for k, v := range m {
			if _, ok := allowed[k]; ok {
				newMap[k] = append([]string(nil), v...)
			}
		}
		if len(newMap) == 0 {
			return errors.New("cannot remove palaces: subaccount " + s.Users[i].Username + " would have no delegated palaces left")
		}
		s.Users[i].PalacePerms = newMap
	}
	return s.saveUnlocked()
}

// ListSubaccounts returns subaccounts owned by parentTenant (password hashes included — sanitize in API).
func (s *Store) ListSubaccounts(parentTenant string) []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []User
	for _, u := range s.Users {
		if u.Role == RoleSubaccount && u.ParentTenant == parentTenant {
			out = append(out, u)
		}
	}
	return out
}

// CreateSubaccount adds a subaccount user under an existing tenant.
func (s *Store) CreateSubaccount(u User, parentTenant, plaintext string) error {
	if u.Username == "" {
		return errors.New("username required")
	}
	if u.Role != RoleSubaccount {
		return errors.New("role must be subaccount")
	}
	parent, ok := s.Get(parentTenant)
	if !ok || parent.Role != RoleTenant {
		return ErrSubaccountParent
	}
	perms := NormalizePalacePerms(u.PalacePerms)
	if err := ValidateSubaccountPalacePerms(parent.Palaces, perms); err != nil {
		return err
	}
	h, err := HashPassword(plaintext)
	if err != nil {
		return err
	}
	u.PasswordBcrypt = h
	u.ParentTenant = parentTenant
	u.PalacePerms = perms
	u.Palaces = nil
	u.Role = RoleSubaccount

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

// UpdateSubaccount replaces palace permissions (and optionally password) for a subaccount of parentTenant.
func (s *Store) UpdateSubaccount(subUsername, parentTenant string, palacePerms map[string][]string, newPasswordOptional *string) error {
	perms := NormalizePalacePerms(palacePerms)
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Users {
		if s.Users[i].Username == subUsername {
			idx = i
			break
		}
	}
	if idx < 0 || s.Users[idx].Role != RoleSubaccount || s.Users[idx].ParentTenant != parentTenant {
		return ErrNotFound
	}
	parent, ok := s.findUserUnlocked(parentTenant)
	if !ok || parent.Role != RoleTenant {
		return ErrSubaccountParent
	}
	if err := ValidateSubaccountPalacePerms(parent.Palaces, perms); err != nil {
		return err
	}
	s.Users[idx].PalacePerms = perms
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

func (s *Store) findUserUnlocked(username string) (User, bool) {
	for _, u := range s.Users {
		if u.Username == username {
			return u, true
		}
	}
	return User{}, false
}

// DeleteSubaccount removes a subaccount if it belongs to parentTenant.
func (s *Store) DeleteSubaccount(subUsername, parentTenant string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i := range s.Users {
		if s.Users[i].Username == subUsername {
			idx = i
			break
		}
	}
	if idx < 0 || s.Users[idx].Role != RoleSubaccount || s.Users[idx].ParentTenant != parentTenant {
		return ErrNotFound
	}
	filtered := s.Users[:0]
	for _, u := range s.Users {
		if u.Username != subUsername {
			filtered = append(filtered, u)
		}
	}
	s.Users = filtered
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
		if u.Username == username {
			continue
		}
		if target.Role == RoleTenant && u.Role == RoleSubaccount && u.ParentTenant == username {
			continue
		}
		filtered = append(filtered, u)
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

// TenantsHavingPalace returns tenant usernames that list this palace in their Palaces slice.
func (s *Store) TenantsHavingPalace(palaceName string) []string {
	if palaceName == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for _, u := range s.Users {
		if u.Role != RoleTenant {
			continue
		}
		for _, p := range u.Palaces {
			if p == palaceName {
				out = append(out, u.Username)
				break
			}
		}
	}
	return out
}

// SingleTenantForPalace returns the tenant username if exactly one tenant has this palace; otherwise "".
func (s *Store) SingleTenantForPalace(palaceName string) string {
	if palaceName == "" {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var hits []string
	for _, u := range s.Users {
		if u.Role != RoleTenant {
			continue
		}
		for _, p := range u.Palaces {
			if p == palaceName {
				hits = append(hits, u.Username)
				break
			}
		}
	}
	if len(hits) == 1 {
		return hits[0]
	}
	return ""
}
