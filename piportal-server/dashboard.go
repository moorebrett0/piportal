package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

//go:embed dashboard/dist
var dashboardFS embed.FS

// handleDashboardAPI routes /api/v1/* requests
func (h *Handler) handleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Public routes
	switch {
	case path == "/api/v1/signup" && r.Method == http.MethodPost:
		h.handleSignup(w, r)
		return
	case path == "/api/v1/login" && r.Method == http.MethodPost:
		h.handleLogin(w, r)
		return
	case path == "/api/v1/logout" && r.Method == http.MethodPost:
		h.handleLogout(w, r)
		return
	}

	// Protected routes
	switch {
	case path == "/api/v1/me" && r.Method == http.MethodGet:
		h.AuthMiddleware(h.handleMe)(w, r)
	case path == "/api/v1/organizations" && r.Method == http.MethodGet:
		h.AuthMiddleware(h.handleListOrgs)(w, r)
	case path == "/api/v1/organizations" && r.Method == http.MethodPost:
		h.AuthMiddleware(h.handleCreateOrg)(w, r)
	case strings.HasPrefix(path, "/api/v1/organizations/") && r.Method == http.MethodPut:
		h.AuthMiddleware(h.handleUpdateOrg)(w, r)
	case strings.HasPrefix(path, "/api/v1/organizations/") && r.Method == http.MethodDelete:
		h.AuthMiddleware(h.handleDeleteOrg)(w, r)
	case path == "/api/v1/devices" && r.Method == http.MethodGet:
		h.AuthMiddleware(h.handleListDevices)(w, r)
	case path == "/api/v1/devices" && r.Method == http.MethodPost:
		h.AuthMiddleware(h.handleCreateDevice)(w, r)
	case path == "/api/v1/devices/claim" && r.Method == http.MethodPost:
		h.AuthMiddleware(h.handleClaimDevice)(w, r)
	case strings.HasPrefix(path, "/api/v1/devices/") && strings.HasSuffix(path, "/terminal") && websocket.IsWebSocketUpgrade(r):
		h.handleTerminalWebSocket(w, r)
	case strings.HasPrefix(path, "/api/v1/devices/") && strings.HasSuffix(path, "/tunnel") && r.Method == http.MethodPut:
		h.AuthMiddleware(h.handleSetTunnelEnabled)(w, r)
	case strings.HasPrefix(path, "/api/v1/devices/") && strings.HasSuffix(path, "/reboot") && r.Method == http.MethodPost:
		h.AuthMiddleware(h.handleRebootDevice)(w, r)
	case strings.HasPrefix(path, "/api/v1/devices/") && strings.HasSuffix(path, "/org") && r.Method == http.MethodPut:
		h.AuthMiddleware(h.handleSetDeviceOrg)(w, r)
	case strings.HasPrefix(path, "/api/v1/devices/") && r.Method == http.MethodGet:
		h.AuthMiddleware(h.handleGetDevice)(w, r)
	case strings.HasPrefix(path, "/api/v1/devices/") && r.Method == http.MethodDelete:
		h.AuthMiddleware(h.handleDeleteDevice)(w, r)
	default:
		jsonError(w, "Not Found", http.StatusNotFound)
	}
}

// serveDashboard serves the React SPA
func (h *Handler) serveDashboard(w http.ResponseWriter, r *http.Request) {
	dist, err := fs.Sub(dashboardFS, "dashboard/dist")
	if err != nil {
		http.Error(w, "Dashboard not available", http.StatusInternalServerError)
		return
	}

	// Try to serve the exact file
	path := strings.TrimPrefix(r.URL.Path, "/dashboard")
	if path == "" || path == "/" {
		path = "/index.html"
	}

	// Check if file exists in embedded FS
	f, err := dist.Open(strings.TrimPrefix(path, "/"))
	if err == nil {
		f.Close()
		// Serve the file
		fileServer := http.FileServer(http.FS(dist))
		// Strip /dashboard prefix so file server finds the files
		http.StripPrefix("/dashboard", fileServer).ServeHTTP(w, r)
		return
	}

	// For SPA routing: serve index.html for any non-file path
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "Dashboard not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(index)
}

// --- Auth Handlers ---

func (h *Handler) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		jsonError(w, "Valid email is required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		log.Printf("Password hash error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	user, err := h.store.CreateUser(req.Email, hash)
	if err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}

	token, err := GenerateJWT(user.ID, h.config.JWTSecret)
	if err != nil {
		log.Printf("JWT generation error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	SetAuthCookie(w, token, h.config.DevMode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
		},
		"token": token,
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		jsonError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByEmail(req.Email)
	if err != nil {
		log.Printf("Login lookup error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if user == nil || !CheckPassword(req.Password, user.PasswordHash) {
		jsonError(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	token, err := GenerateJWT(user.ID, h.config.JWTSecret)
	if err != nil {
		log.Printf("JWT generation error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	SetAuthCookie(w, token, h.config.DevMode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
		},
		"token": token,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	ClearAuthCookie(w, h.config.DevMode)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// --- Protected Handlers ---

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)

	count, err := h.store.CountDevicesByUser(user.ID)
	if err != nil {
		log.Printf("Count devices error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           user.ID,
		"email":        user.Email,
		"created_at":   user.CreatedAt,
		"device_count": count,
	})
}

func (h *Handler) handleListDevices(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)

	// Check for org_id filter in query params
	var devices []*Device
	var err error
	orgIDParam := r.URL.Query().Get("org_id")
	if orgIDParam != "" {
		// Filter by specific org
		devices, err = h.store.ListDevicesByUserAndOrg(user.ID, &orgIDParam)
	} else {
		// All devices for user
		devices, err = h.store.ListDevicesByUser(user.ID)
	}
	if err != nil {
		log.Printf("List devices error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Build response with bandwidth info and metrics
	type deviceResponse struct {
		ID            string   `json:"id"`
		Subdomain     string   `json:"subdomain"`
		URL           string   `json:"url"`
		Tier          string   `json:"tier"`
		IsOnline      bool     `json:"is_online"`
		TunnelEnabled bool     `json:"tunnel_enabled"`
		CreatedAt     string   `json:"created_at"`
		LastSeenAt    string   `json:"last_seen_at,omitempty"`
		BytesIn       int64    `json:"bytes_in"`
		BytesOut      int64    `json:"bytes_out"`
		BytesTotal    int64    `json:"bytes_total"`
		Limit         int64    `json:"limit"`
		OrgID         string   `json:"org_id,omitempty"`
		OrgName       string   `json:"org_name,omitempty"`
		CPUTemp       *float64 `json:"cpu_temp,omitempty"`
		MemTotal      *uint64  `json:"mem_total,omitempty"`
		MemFree       *uint64  `json:"mem_free,omitempty"`
		DiskTotal     *uint64  `json:"disk_total,omitempty"`
		DiskFree      *uint64  `json:"disk_free,omitempty"`
		DevUptime     *int64   `json:"uptime,omitempty"`
		LoadAvg       *float64 `json:"load_avg,omitempty"`
	}

	// Build org name lookup map
	orgs, _ := h.store.ListOrganizationsByUser(user.ID)
	orgNames := make(map[string]string)
	for _, org := range orgs {
		orgNames[org.ID] = org.Name
	}

	var result []deviceResponse
	for _, d := range devices {
		dr := deviceResponse{
			ID:            d.ID,
			Subdomain:     d.Subdomain,
			URL:           "https://" + d.Subdomain + "." + h.config.BaseDomain,
			Tier:          d.Tier,
			IsOnline:      d.IsOnline,
			TunnelEnabled: d.TunnelEnabled,
			CreatedAt:     d.CreatedAt.Format("2006-01-02T15:04:05Z"),
			OrgID:         d.OrgID,
		}
		if d.OrgID != "" {
			dr.OrgName = orgNames[d.OrgID]
		}
		if !d.LastSeenAt.IsZero() {
			dr.LastSeenAt = d.LastSeenAt.Format("2006-01-02T15:04:05Z")
		}

		usage, err := h.store.GetMonthlyUsage(d.ID)
		if err == nil {
			dr.BytesIn = usage.BytesIn
			dr.BytesOut = usage.BytesOut
			dr.BytesTotal = usage.BytesIn + usage.BytesOut
		}
		limit, err := h.store.GetBandwidthLimit(d.ID)
		if err == nil {
			dr.Limit = limit
		}

		// Include metrics if device is online and has an active tunnel
		if d.IsOnline {
			if tunnel := h.tunnels.GetTunnel(d.Subdomain); tunnel != nil {
				if m := tunnel.GetMetrics(); m != nil {
					dr.CPUTemp = &m.CPUTemp
					dr.MemTotal = &m.MemTotal
					dr.MemFree = &m.MemFree
					dr.DiskTotal = &m.DiskTotal
					dr.DiskFree = &m.DiskFree
					dr.DevUptime = &m.Uptime
					dr.LoadAvg = &m.LoadAvg
				}
			}
		}

		result = append(result, dr)
	}

	if result == nil {
		result = []deviceResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	deviceID := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")

	device, err := h.store.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("Get device error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil || device.UserID != user.ID {
		jsonError(w, "Device not found", http.StatusNotFound)
		return
	}

	usage, _ := h.store.GetMonthlyUsage(device.ID)
	limit, _ := h.store.GetBandwidthLimit(device.ID)

	resp := map[string]interface{}{
		"id":             device.ID,
		"subdomain":      device.Subdomain,
		"url":            "https://" + device.Subdomain + "." + h.config.BaseDomain,
		"tier":           device.Tier,
		"is_online":      device.IsOnline,
		"tunnel_enabled": device.TunnelEnabled,
		"created_at":     device.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if !device.LastSeenAt.IsZero() {
		resp["last_seen_at"] = device.LastSeenAt.Format("2006-01-02T15:04:05Z")
	}
	if usage != nil {
		resp["bytes_in"] = usage.BytesIn
		resp["bytes_out"] = usage.BytesOut
		resp["bytes_total"] = usage.BytesIn + usage.BytesOut
	}
	if limit > 0 {
		resp["limit"] = limit
	}

	// Include org info
	if device.OrgID != "" {
		resp["org_id"] = device.OrgID
		if org, err := h.store.GetOrganizationByID(device.OrgID); err == nil && org != nil {
			resp["org_name"] = org.Name
		}
	}

	// Include metrics if device is online
	if device.IsOnline {
		if tunnel := h.tunnels.GetTunnel(device.Subdomain); tunnel != nil {
			if m := tunnel.GetMetrics(); m != nil {
				resp["cpu_temp"] = m.CPUTemp
				resp["mem_total"] = m.MemTotal
				resp["mem_free"] = m.MemFree
				resp["disk_total"] = m.DiskTotal
				resp["disk_free"] = m.DiskFree
				resp["uptime"] = m.Uptime
				resp["load_avg"] = m.LoadAvg
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)

	var req struct {
		Subdomain string `json:"subdomain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Subdomain == "" {
		jsonError(w, "subdomain is required", http.StatusBadRequest)
		return
	}

	device, err := h.store.CreateDevice(req.Subdomain, user.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"id":        device.ID,
		"token":     device.Token,
		"subdomain": device.Subdomain,
		"url":       "https://" + device.Subdomain + "." + h.config.BaseDomain,
	})
}

func (h *Handler) handleClaimDevice(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		jsonError(w, "token is required", http.StatusBadRequest)
		return
	}

	device, err := h.store.GetDeviceByTokenValue(req.Token)
	if err != nil {
		log.Printf("Claim device lookup error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil {
		jsonError(w, "Invalid token", http.StatusNotFound)
		return
	}
	if device.UserID != "" {
		jsonError(w, "Device is already claimed", http.StatusConflict)
		return
	}

	if err := h.store.AssignDeviceToUser(device.ID, user.ID); err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"id":        device.ID,
		"subdomain": device.Subdomain,
		"url":       "https://" + device.Subdomain + "." + h.config.BaseDomain,
	})
}

func (h *Handler) handleRebootDevice(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	// Path: /api/v1/devices/{id}/reboot
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), "/")
	if len(parts) < 2 {
		jsonError(w, "Invalid path", http.StatusBadRequest)
		return
	}
	deviceID := parts[0]

	device, err := h.store.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("Reboot device error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil || device.UserID != user.ID {
		jsonError(w, "Device not found", http.StatusNotFound)
		return
	}

	tunnel := h.tunnels.GetTunnel(device.Subdomain)
	if tunnel == nil {
		jsonError(w, "Device is offline", http.StatusConflict)
		return
	}

	if err := tunnel.SendCommand("reboot"); err != nil {
		log.Printf("Failed to send reboot command to %s: %v", device.Subdomain, err)
		jsonError(w, "Failed to send reboot command", http.StatusInternalServerError)
		return
	}

	log.Printf("Reboot command sent to device %s (%s)", device.Subdomain, device.ID[:8])

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (h *Handler) handleSetTunnelEnabled(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	// Path: /api/v1/devices/{id}/tunnel
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), "/")
	if len(parts) < 2 {
		jsonError(w, "Invalid path", http.StatusBadRequest)
		return
	}
	deviceID := parts[0]

	device, err := h.store.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("Set tunnel enabled error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil || device.UserID != user.ID {
		jsonError(w, "Device not found", http.StatusNotFound)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := h.store.SetTunnelEnabled(deviceID, req.Enabled); err != nil {
		log.Printf("Set tunnel enabled error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"tunnel_enabled": req.Enabled,
	})
}

func (h *Handler) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	deviceID := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")

	device, err := h.store.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("Delete device error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil || device.UserID != user.ID {
		jsonError(w, "Device not found", http.StatusNotFound)
		return
	}

	// Disconnect active tunnel if any
	if tunnel := h.tunnels.GetTunnel(device.Subdomain); tunnel != nil {
		tunnel.Close()
	}

	if err := h.store.DeleteDevice(device.ID); err != nil {
		log.Printf("Delete device error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// --- Organization Handlers ---

func (h *Handler) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)

	orgs, err := h.store.ListOrganizationsByUser(user.ID)
	if err != nil {
		log.Printf("List orgs error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	type orgResponse struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}

	var result []orgResponse
	for _, org := range orgs {
		result = append(result, orgResponse{
			ID:        org.ID,
			Name:      org.Name,
			CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	if result == nil {
		result = []orgResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	org, err := h.store.CreateOrganization(req.Name, user.ID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         org.ID,
		"name":       org.Name,
		"created_at": org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *Handler) handleUpdateOrg(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	orgID := strings.TrimPrefix(r.URL.Path, "/api/v1/organizations/")

	// Verify ownership
	org, err := h.store.GetOrganizationByID(orgID)
	if err != nil {
		log.Printf("Update org error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if org == nil || org.UserID != user.ID {
		jsonError(w, "Organization not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateOrganization(orgID, req.Name); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      orgID,
		"name":    req.Name,
	})
}

func (h *Handler) handleDeleteOrg(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	orgID := strings.TrimPrefix(r.URL.Path, "/api/v1/organizations/")

	// Verify ownership
	org, err := h.store.GetOrganizationByID(orgID)
	if err != nil {
		log.Printf("Delete org error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if org == nil || org.UserID != user.ID {
		jsonError(w, "Organization not found", http.StatusNotFound)
		return
	}

	if err := h.store.DeleteOrganization(orgID); err != nil {
		log.Printf("Delete org error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (h *Handler) handleSetDeviceOrg(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	// Path: /api/v1/devices/{id}/org
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), "/")
	if len(parts) < 2 {
		jsonError(w, "Invalid path", http.StatusBadRequest)
		return
	}
	deviceID := parts[0]

	device, err := h.store.GetDeviceByID(deviceID)
	if err != nil {
		log.Printf("Set device org error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if device == nil || device.UserID != user.ID {
		jsonError(w, "Device not found", http.StatusNotFound)
		return
	}

	var req struct {
		OrgID *string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// If org_id is provided, verify it belongs to the user
	if req.OrgID != nil && *req.OrgID != "" {
		org, err := h.store.GetOrganizationByID(*req.OrgID)
		if err != nil {
			log.Printf("Set device org error: %v", err)
			jsonError(w, "Internal error", http.StatusInternalServerError)
			return
		}
		if org == nil || org.UserID != user.ID {
			jsonError(w, "Organization not found", http.StatusNotFound)
			return
		}
	}

	if err := h.store.SetDeviceOrganization(deviceID, req.OrgID); err != nil {
		log.Printf("Set device org error: %v", err)
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"org_id":  req.OrgID,
	})
}
