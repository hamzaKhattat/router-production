package provider

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "strings"
    "sync"
    "time"
    
    "github.com/router-production/internal/database"
    "github.com/router-production/internal/models"
)

type Manager struct {
    db            *database.DB
    providers     map[string]*models.Provider
    providerDIDs  map[string][]string
    mu            sync.RWMutex
    asteriskGen   *AsteriskConfigGenerator
}

func NewManager(db *database.DB) *Manager {
    m := &Manager{
        db:           db,
        providers:    make(map[string]*models.Provider),
        providerDIDs: make(map[string][]string),
        asteriskGen:  NewAsteriskConfigGenerator(),
    }
    
    // Load existing providers
    m.LoadProviders()
    
    return m
}

func (m *Manager) AddProvider(p *models.Provider) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    fmt.Println(time.Now())
    fmt.Println(sql.ErrNoRows)
    
    // Validate provider
    if p.Name == "" || p.Host == "" {
        return fmt.Errorf("provider name and host are required")
    }
    
    if p.Port == 0 {
        p.Port = 5060
    }
    
    if p.Transport == "" {
        p.Transport = "udp"
    }
    
    if len(p.Codecs) == 0 {
        p.Codecs = []string{"ulaw", "alaw"}
    }
    
    // Store in database
    codecsJSON, _ := json.Marshal(p.Codecs)
    result, err := m.db.Exec(`
        INSERT INTO providers (name, host, port, username, password, realm, transport, codecs, max_channels, active, country)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
        host=VALUES(host), port=VALUES(port), username=VALUES(username), 
        password=VALUES(password), realm=VALUES(realm), transport=VALUES(transport),
        codecs=VALUES(codecs), max_channels=VALUES(max_channels), 
        active=VALUES(active), country=VALUES(country), updated_at=NOW()
    `, p.Name, p.Host, p.Port, p.Username, p.Password, p.Realm, p.Transport, codecsJSON, p.MaxChannels, p.Active, p.Country)
    
    if err != nil {
        return fmt.Errorf("failed to add provider: %w", err)
    }
    
    id, _ := result.LastInsertId()
    p.ID = int(id)
    
    // Store in memory
    m.providers[p.Name] = p
    
    // Generate Asterisk configuration
    if err := m.asteriskGen.GenerateProviderConfig(p); err != nil {
        log.Printf("Warning: Failed to generate Asterisk config for %s: %v", p.Name, err)
    }
    
    log.Printf("Provider %s added successfully", p.Name)
    return nil
}

func (m *Manager) AddDIDs(providerName string, dids []string, country string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    provider, exists := m.providers[providerName]
    if !exists {
        return fmt.Errorf("provider %s not found", providerName)
    }
    
    // Prepare bulk insert
    values := make([]string, 0, len(dids))
    args := make([]interface{}, 0, len(dids)*4)
    
    for _, did := range dids {
        values = append(values, "(?, ?, ?, ?)")
        args = append(args, did, provider.ID, provider.Name, country)
    }
    
    query := fmt.Sprintf(`
        INSERT INTO dids (did, provider_id, provider_name, country)
        VALUES %s
        ON DUPLICATE KEY UPDATE provider_id=VALUES(provider_id), 
        provider_name=VALUES(provider_name), country=VALUES(country)
    `, strings.Join(values, ","))
    
    if _, err := m.db.Exec(query, args...); err != nil {
        return fmt.Errorf("failed to add DIDs: %w", err)
    }
    
    // Update memory cache
    m.providerDIDs[providerName] = append(m.providerDIDs[providerName], dids...)
    
    log.Printf("Added %d DIDs to provider %s", len(dids), providerName)
    return nil
}

func (m *Manager) GetAvailableDID(providerName string) (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    var query string
    var args []interface{}
    
    if providerName != "" {
        // Get DID from specific provider
        query = `
            SELECT d.did 
            FROM dids d
            JOIN providers p ON d.provider_id = p.id
            WHERE d.in_use = 0 AND p.name = ? AND p.active = 1
            ORDER BY RAND() 
            LIMIT 1
            FOR UPDATE
        `
        args = []interface{}{providerName}
    } else {
        // Get DID from any active provider
        query = `
            SELECT d.did 
            FROM dids d
            JOIN providers p ON d.provider_id = p.id
            WHERE d.in_use = 0 AND p.active = 1
            ORDER BY RAND() 
            LIMIT 1
            FOR UPDATE
        `
    }
    
    var did string
    err := m.db.QueryRow(query, args...).Scan(&did)
    if err != nil {
        return "", fmt.Errorf("no available DIDs: %w", err)
    }
    
    return did, nil
}

func (m *Manager) LoadProviders() error {
    rows, err := m.db.Query(`
        SELECT id, name, host, port, username, password, realm, transport, 
               codecs, max_channels, active, country
        FROM providers
        WHERE active = 1
    `)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for rows.Next() {
        p := &models.Provider{}
        var codecsJSON []byte
        
        err := rows.Scan(&p.ID, &p.Name, &p.Host, &p.Port, &p.Username, 
            &p.Password, &p.Realm, &p.Transport, &codecsJSON, 
            &p.MaxChannels, &p.Active, &p.Country)
        
        if err != nil {
            log.Printf("Error loading provider: %v", err)
            continue
        }
        
        json.Unmarshal(codecsJSON, &p.Codecs)
        m.providers[p.Name] = p
        
        // Load DIDs for this provider
        m.loadProviderDIDs(p.Name, p.ID)
    }
    
    log.Printf("Loaded %d providers", len(m.providers))
    return nil
}

func (m *Manager) loadProviderDIDs(providerName string, providerID int) {
    rows, err := m.db.Query("SELECT did FROM dids WHERE provider_id = ?", providerID)
    if err != nil {
        return
    }
    defer rows.Close()
    
    dids := []string{}
    for rows.Next() {
        var did string
        if err := rows.Scan(&did); err == nil {
            dids = append(dids, did)
        }
    }
    
    m.providerDIDs[providerName] = dids
}

func (m *Manager) GetProvider(name string) (*models.Provider, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    provider, exists := m.providers[name]
    if !exists {
        return nil, fmt.Errorf("provider %s not found", name)
    }
    
    return provider, nil
}

func (m *Manager) ListProviders() []*models.Provider {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    providers := make([]*models.Provider, 0, len(m.providers))
    for _, p := range m.providers {
        providers = append(providers, p)
    }
    
    return providers
}

func (m *Manager) GetProviderStats(name string) (map[string]interface{}, error) {
    stats := make(map[string]interface{})
    
    // Get provider info
    provider, err := m.GetProvider(name)
    if err != nil {
        return nil, err
    }
    
    stats["provider"] = provider
    
    // Get DID counts
    var totalDIDs, usedDIDs int
    err = m.db.QueryRow(`
        SELECT COUNT(*), SUM(CASE WHEN in_use = 1 THEN 1 ELSE 0 END)
        FROM dids WHERE provider_id = ?
    `, provider.ID).Scan(&totalDIDs, &usedDIDs)
    
    if err == nil {
        stats["total_dids"] = totalDIDs
        stats["used_dids"] = usedDIDs
        stats["available_dids"] = totalDIDs - usedDIDs
    }
    
    // Get call statistics
    var totalCalls, activeCalls int
    err = m.db.QueryRow(`
        SELECT COUNT(*), SUM(CASE WHEN status IN ('ACTIVE', 'FORWARDED') THEN 1 ELSE 0 END)
        FROM call_records 
        WHERE provider_id = ? AND DATE(start_time) = CURDATE()
    `, provider.ID).Scan(&totalCalls, &activeCalls)
    
    if err == nil {
        stats["calls_today"] = totalCalls
        stats["active_calls"] = activeCalls
    }
    
    return stats, nil
}
