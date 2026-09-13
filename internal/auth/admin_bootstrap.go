package auth

import "fmt"

// BootstrapAdmin seeds the single admin user from the given credentials, but
// only if the users table is currently empty. On later boots this is a
// no-op, so ADMIN_USERNAME/ADMIN_PASSWORD can safely stay set in the
// systemd EnvironmentFile without resetting the password every restart.
func BootstrapAdmin(store *Store, username, password string) error {
	count, err := store.UserCount()
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	if err := store.CreateUser(username, hash); err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	return nil
}
