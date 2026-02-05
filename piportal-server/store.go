package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Bandwidth limits (in bytes)
const (
	FreeTierBandwidth = 1 * 1024 * 1024 * 1024  // 1 GB/month
	ProTierBandwidth  = 100 * 1024 * 1024 * 1024 // 100 GB/month
)

// Store handles persistent storage
type Store struct {
	db *sql.DB
}

// User represents a registered dashboard user
type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Device represents a registered device
type Device struct {
	ID            string
	Token         string // The actual token (only returned on creation)
	TokenHash     string // Stored hash
	Subdomain     string
	Tier          string // "free" or "pro"
	UserID        string // Owner user ID (empty if unclaimed)
	OrgID         string // Organization ID (empty if unassigned)
	CreatedAt     time.Time
	LastSeenAt    time.Time
	IsOnline      bool
	TunnelEnabled bool
}

// Organization represents a named device group owned by a user
type Organization struct {
	ID        string
	Name      string
	UserID    string
	CreatedAt time.Time
}

// Usage represents monthly bandwidth usage
type Usage struct {
	DeviceID string
	Month    string // YYYY-MM format
	BytesIn  int64
	BytesOut int64
}

// NewStore creates a new store with SQLite
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, err
	}

	return store, nil
}

// migrate creates the database schema
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		token_hash TEXT UNIQUE NOT NULL,
		subdomain TEXT UNIQUE NOT NULL,
		tier TEXT DEFAULT 'free',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen_at DATETIME,
		is_online BOOLEAN DEFAULT FALSE
	);

	CREATE INDEX IF NOT EXISTS idx_devices_token_hash ON devices(token_hash);
	CREATE INDEX IF NOT EXISTS idx_devices_subdomain ON devices(subdomain);

	CREATE TABLE IF NOT EXISTS usage (
		device_id TEXT NOT NULL,
		month TEXT NOT NULL,
		bytes_in INTEGER DEFAULT 0,
		bytes_out INTEGER DEFAULT 0,
		PRIMARY KEY (device_id, month),
		FOREIGN KEY (device_id) REFERENCES devices(id)
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Add user_id column to devices (ignore error if already exists)
	s.db.Exec("ALTER TABLE devices ADD COLUMN user_id TEXT REFERENCES users(id)")

	// Add tunnel_enabled column (default FALSE — new devices start with forwarding disabled)
	s.db.Exec("ALTER TABLE devices ADD COLUMN tunnel_enabled BOOLEAN DEFAULT FALSE")

	// Organizations table
	s.db.Exec(`CREATE TABLE IF NOT EXISTS organizations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		user_id TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_organizations_user ON organizations(user_id)")

	// Add org_id column to devices
	s.db.Exec("ALTER TABLE devices ADD COLUMN org_id TEXT REFERENCES organizations(id)")

	return nil
}

// CreateDevice creates a new device with a random token.
// If userID is non-empty, the device is owned by that user.
func (s *Store) CreateDevice(subdomain string, userID string) (*Device, error) {
	subdomain = strings.ToLower(strings.TrimSpace(subdomain))
	if err := validateSubdomain(subdomain); err != nil {
		return nil, err
	}

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM devices WHERE subdomain = ?)", subdomain).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("subdomain '%s' is already taken", subdomain)
	}

	id := generateID()
	token := generateToken()
	tokenHash := hashToken(token)

	if userID != "" {
		_, err = s.db.Exec(
			"INSERT INTO devices (id, token_hash, subdomain, tier, user_id) VALUES (?, ?, ?, 'free', ?)",
			id, tokenHash, subdomain, userID,
		)
	} else {
		_, err = s.db.Exec(
			"INSERT INTO devices (id, token_hash, subdomain, tier) VALUES (?, ?, ?, 'free')",
			id, tokenHash, subdomain,
		)
	}
	if err != nil {
		return nil, err
	}

	return &Device{
		ID:        id,
		Token:     token,
		Subdomain: subdomain,
		Tier:      "free",
		UserID:    userID,
		CreatedAt: time.Now(),
	}, nil
}

// GetDeviceByToken looks up a device by its token
func (s *Store) GetDeviceByToken(token string) (*Device, error) {
	tokenHash := hashToken(token)

	var device Device
	var lastSeen sql.NullTime
	var tier sql.NullString
	var orgID sql.NullString
	err := s.db.QueryRow(
		"SELECT id, subdomain, tier, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE token_hash = ?",
		tokenHash,
	).Scan(&device.ID, &device.Subdomain, &tier, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &orgID)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if lastSeen.Valid {
		device.LastSeenAt = lastSeen.Time
	}
	device.TokenHash = tokenHash
	device.Tier = "free"
	if tier.Valid {
		device.Tier = tier.String
	}
	if orgID.Valid {
		device.OrgID = orgID.String
	}

	return &device, nil
}

// GetDeviceBySubdomain looks up a device by subdomain
func (s *Store) GetDeviceBySubdomain(subdomain string) (*Device, error) {
	var device Device
	var lastSeen sql.NullTime
	var tier sql.NullString
	var orgID sql.NullString
	err := s.db.QueryRow(
		"SELECT id, subdomain, tier, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE subdomain = ?",
		subdomain,
	).Scan(&device.ID, &device.Subdomain, &tier, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &orgID)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if lastSeen.Valid {
		device.LastSeenAt = lastSeen.Time
	}
	device.Tier = "free"
	if tier.Valid {
		device.Tier = tier.String
	}
	if orgID.Valid {
		device.OrgID = orgID.String
	}

	return &device, nil
}

// UpdateDeviceStatus updates the online status
func (s *Store) UpdateDeviceStatus(deviceID string, online bool) error {
	_, err := s.db.Exec(
		"UPDATE devices SET is_online = ?, last_seen_at = CURRENT_TIMESTAMP WHERE id = ?",
		online, deviceID,
	)
	return err
}

// UpgradeDevice upgrades a device to pro tier
func (s *Store) UpgradeDevice(deviceID string) error {
	_, err := s.db.Exec("UPDATE devices SET tier = 'pro' WHERE id = ?", deviceID)
	return err
}

// SetTunnelEnabled enables or disables tunnel forwarding for a device
func (s *Store) SetTunnelEnabled(deviceID string, enabled bool) error {
	_, err := s.db.Exec("UPDATE devices SET tunnel_enabled = ? WHERE id = ?", enabled, deviceID)
	return err
}

// --- Bandwidth Tracking ---

// currentMonth returns the current month in YYYY-MM format
func currentMonth() string {
	return time.Now().Format("2006-01")
}

// AddBandwidth records bandwidth usage for a device
func (s *Store) AddBandwidth(deviceID string, bytesIn, bytesOut int64) error {
	month := currentMonth()

	// Upsert: insert or update
	_, err := s.db.Exec(`
		INSERT INTO usage (device_id, month, bytes_in, bytes_out)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(device_id, month) DO UPDATE SET
			bytes_in = bytes_in + excluded.bytes_in,
			bytes_out = bytes_out + excluded.bytes_out
	`, deviceID, month, bytesIn, bytesOut)

	return err
}

// GetMonthlyUsage returns bandwidth usage for the current month
func (s *Store) GetMonthlyUsage(deviceID string) (*Usage, error) {
	month := currentMonth()

	var usage Usage
	err := s.db.QueryRow(
		"SELECT device_id, month, bytes_in, bytes_out FROM usage WHERE device_id = ? AND month = ?",
		deviceID, month,
	).Scan(&usage.DeviceID, &usage.Month, &usage.BytesIn, &usage.BytesOut)

	if err == sql.ErrNoRows {
		return &Usage{DeviceID: deviceID, Month: month, BytesIn: 0, BytesOut: 0}, nil
	}
	if err != nil {
		return nil, err
	}

	return &usage, nil
}

// GetBandwidthLimit returns the bandwidth limit for a device based on tier
func (s *Store) GetBandwidthLimit(deviceID string) (int64, error) {
	var tier sql.NullString
	err := s.db.QueryRow("SELECT tier FROM devices WHERE id = ?", deviceID).Scan(&tier)
	if err != nil {
		return 0, err
	}

	if tier.Valid && tier.String == "pro" {
		return ProTierBandwidth, nil
	}
	return FreeTierBandwidth, nil
}

// IsOverBandwidthLimit checks if a device has exceeded its monthly limit
func (s *Store) IsOverBandwidthLimit(deviceID string) (bool, int64, int64, error) {
	usage, err := s.GetMonthlyUsage(deviceID)
	if err != nil {
		return false, 0, 0, err
	}

	limit, err := s.GetBandwidthLimit(deviceID)
	if err != nil {
		return false, 0, 0, err
	}

	totalUsed := usage.BytesIn + usage.BytesOut
	return totalUsed >= limit, totalUsed, limit, nil
}

// --- User Methods ---

// CreateUser creates a new user account
func (s *Store) CreateUser(email, passwordHash string) (*User, error) {
	id := generateID()
	_, err := s.db.Exec(
		"INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)",
		id, email, passwordHash,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("email already registered")
		}
		return nil, err
	}
	return &User{ID: id, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now()}, nil
}

// GetUserByEmail looks up a user by email
func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, email, password_hash, created_at FROM users WHERE email = ?", email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID looks up a user by ID
func (s *Store) GetUserByID(id string) (*User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, email, password_hash, created_at FROM users WHERE id = ?", id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ListDevicesByUser returns all devices owned by a user
func (s *Store) ListDevicesByUser(userID string) ([]*Device, error) {
	rows, err := s.db.Query(
		"SELECT id, subdomain, tier, user_id, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var lastSeen sql.NullTime
		var tier sql.NullString
		var uid sql.NullString
		var orgID sql.NullString
		if err := rows.Scan(&device.ID, &device.Subdomain, &tier, &uid, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &orgID); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			device.LastSeenAt = lastSeen.Time
		}
		device.Tier = "free"
		if tier.Valid {
			device.Tier = tier.String
		}
		if uid.Valid {
			device.UserID = uid.String
		}
		if orgID.Valid {
			device.OrgID = orgID.String
		}
		devices = append(devices, &device)
	}
	return devices, nil
}

// GetDeviceByID looks up a device by its ID
func (s *Store) GetDeviceByID(id string) (*Device, error) {
	var device Device
	var lastSeen sql.NullTime
	var tier sql.NullString
	var uid sql.NullString
	var orgID sql.NullString
	err := s.db.QueryRow(
		"SELECT id, token_hash, subdomain, tier, user_id, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE id = ?", id,
	).Scan(&device.ID, &device.TokenHash, &device.Subdomain, &tier, &uid, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &orgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		device.LastSeenAt = lastSeen.Time
	}
	device.Tier = "free"
	if tier.Valid {
		device.Tier = tier.String
	}
	if uid.Valid {
		device.UserID = uid.String
	}
	if orgID.Valid {
		device.OrgID = orgID.String
	}
	return &device, nil
}

// AssignDeviceToUser sets the user_id on a device (claiming)
func (s *Store) AssignDeviceToUser(deviceID, userID string) error {
	result, err := s.db.Exec(
		"UPDATE devices SET user_id = ? WHERE id = ? AND user_id IS NULL",
		userID, deviceID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found or already claimed")
	}
	return nil
}

// DeleteDevice removes a device
func (s *Store) DeleteDevice(deviceID string) error {
	_, err := s.db.Exec("DELETE FROM usage WHERE device_id = ?", deviceID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("DELETE FROM devices WHERE id = ?", deviceID)
	return err
}

// CountDevicesByUser returns the number of devices owned by a user
func (s *Store) CountDevicesByUser(userID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM devices WHERE user_id = ?", userID).Scan(&count)
	return count, err
}

// GetDeviceByTokenValue looks up a device by its raw token (for claiming)
func (s *Store) GetDeviceByTokenValue(token string) (*Device, error) {
	tokenHash := hashToken(token)
	var device Device
	var lastSeen sql.NullTime
	var tier sql.NullString
	var uid sql.NullString
	var orgID sql.NullString
	err := s.db.QueryRow(
		"SELECT id, subdomain, tier, user_id, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE token_hash = ?",
		tokenHash,
	).Scan(&device.ID, &device.Subdomain, &tier, &uid, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &orgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		device.LastSeenAt = lastSeen.Time
	}
	device.Tier = "free"
	if tier.Valid {
		device.Tier = tier.String
	}
	if uid.Valid {
		device.UserID = uid.String
	}
	if orgID.Valid {
		device.OrgID = orgID.String
	}
	return &device, nil
}

// ListDevices returns all devices
func (s *Store) ListDevices() ([]*Device, error) {
	rows, err := s.db.Query(
		"SELECT id, subdomain, tier, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var lastSeen sql.NullTime
		var tier sql.NullString
		var orgID sql.NullString
		if err := rows.Scan(&device.ID, &device.Subdomain, &tier, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &orgID); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			device.LastSeenAt = lastSeen.Time
		}
		device.Tier = "free"
		if tier.Valid {
			device.Tier = tier.String
		}
		if orgID.Valid {
			device.OrgID = orgID.String
		}
		devices = append(devices, &device)
	}

	return devices, nil
}

// --- Organization Methods ---

// CreateOrganization creates a new organization for a user
func (s *Store) CreateOrganization(name, userID string) (*Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("organization name is required")
	}
	if len(name) > 50 {
		return nil, fmt.Errorf("organization name must be 50 characters or less")
	}

	id := generateID()
	_, err := s.db.Exec(
		"INSERT INTO organizations (id, name, user_id) VALUES (?, ?, ?)",
		id, name, userID,
	)
	if err != nil {
		return nil, err
	}

	return &Organization{ID: id, Name: name, UserID: userID, CreatedAt: time.Now()}, nil
}

// ListOrganizationsByUser returns all organizations owned by a user
func (s *Store) ListOrganizationsByUser(userID string) ([]*Organization, error) {
	rows, err := s.db.Query(
		"SELECT id, name, user_id, created_at FROM organizations WHERE user_id = ? ORDER BY name ASC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		var org Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.UserID, &org.CreatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, &org)
	}
	return orgs, nil
}

// GetOrganizationByID looks up an organization by ID
func (s *Store) GetOrganizationByID(id string) (*Organization, error) {
	var org Organization
	err := s.db.QueryRow(
		"SELECT id, name, user_id, created_at FROM organizations WHERE id = ?", id,
	).Scan(&org.ID, &org.Name, &org.UserID, &org.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// UpdateOrganization renames an organization
func (s *Store) UpdateOrganization(orgID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("organization name is required")
	}
	if len(name) > 50 {
		return fmt.Errorf("organization name must be 50 characters or less")
	}

	result, err := s.db.Exec("UPDATE organizations SET name = ? WHERE id = ?", name, orgID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("organization not found")
	}
	return nil
}

// DeleteOrganization deletes an organization and unassigns its devices
func (s *Store) DeleteOrganization(orgID string) error {
	// First unassign all devices from this org
	_, err := s.db.Exec("UPDATE devices SET org_id = NULL WHERE org_id = ?", orgID)
	if err != nil {
		return err
	}

	// Then delete the org
	_, err = s.db.Exec("DELETE FROM organizations WHERE id = ?", orgID)
	return err
}

// SetDeviceOrganization sets or clears a device's organization
func (s *Store) SetDeviceOrganization(deviceID string, orgID *string) error {
	var err error
	if orgID == nil || *orgID == "" {
		_, err = s.db.Exec("UPDATE devices SET org_id = NULL WHERE id = ?", deviceID)
	} else {
		_, err = s.db.Exec("UPDATE devices SET org_id = ? WHERE id = ?", *orgID, deviceID)
	}
	return err
}

// ListDevicesByUserAndOrg returns devices filtered by user and optionally by org
// If orgID is nil, returns all devices for the user
// If orgID is empty string, returns devices with no org assigned
func (s *Store) ListDevicesByUserAndOrg(userID string, orgID *string) ([]*Device, error) {
	var rows *sql.Rows
	var err error

	if orgID == nil {
		// All devices for user
		rows, err = s.db.Query(
			"SELECT id, subdomain, tier, user_id, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE user_id = ? ORDER BY created_at DESC",
			userID,
		)
	} else {
		// Devices filtered by org (or NULL org if empty string)
		rows, err = s.db.Query(
			"SELECT id, subdomain, tier, user_id, created_at, last_seen_at, is_online, tunnel_enabled, org_id FROM devices WHERE user_id = ? AND org_id = ? ORDER BY created_at DESC",
			userID, *orgID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []*Device
	for rows.Next() {
		var device Device
		var lastSeen sql.NullTime
		var tier sql.NullString
		var uid sql.NullString
		var oid sql.NullString
		if err := rows.Scan(&device.ID, &device.Subdomain, &tier, &uid, &device.CreatedAt, &lastSeen, &device.IsOnline, &device.TunnelEnabled, &oid); err != nil {
			return nil, err
		}
		if lastSeen.Valid {
			device.LastSeenAt = lastSeen.Time
		}
		device.Tier = "free"
		if tier.Valid {
			device.Tier = tier.String
		}
		if uid.Valid {
			device.UserID = uid.String
		}
		if oid.Valid {
			device.OrgID = oid.String
		}
		devices = append(devices, &device)
	}
	return devices, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// --- Helpers ---

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return "pp_" + hex.EncodeToString(b)
}

func hashToken(token string) string {
	return token
}

func validateSubdomain(s string) error {
	if len(s) < 3 || len(s) > 30 {
		return fmt.Errorf("subdomain must be 3-30 characters")
	}

	for i, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (c == '-' && i > 0 && i < len(s)-1)) {
			return fmt.Errorf("subdomain must be lowercase alphanumeric with hyphens (no leading/trailing hyphens)")
		}
	}

	reserved := map[string]bool{
		"www": true, "api": true, "app": true, "admin": true,
		"mail": true, "ftp": true, "ssh": true, "tunnel": true,
		"dev": true, "staging": true, "test": true,
	}
	if reserved[s] {
		return fmt.Errorf("'%s' is a reserved subdomain", s)
	}

	return nil
}

// FormatBytes returns a human-readable byte size
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
