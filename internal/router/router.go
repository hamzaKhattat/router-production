package router

import (
    "fmt"
    "log"
    "strings"
    "sync"
    "time"
    
    "github.com/router-production/internal/database"
    "github.com/router-production/internal/models"
    "github.com/router-production/internal/provider"
)

type Router struct {
    db              *database.DB
    providerManager *provider.Manager
    mu              sync.RWMutex
    activeCallsMap  map[string]*models.CallRecord
    didToCallMap    map[string]string
    recordingPath   string
    routingRules    map[string]string // country -> provider mapping
}

func NewRouter(db *database.DB, pm *provider.Manager) *Router {
    r := &Router{
        db:              db,
        providerManager: pm,
        activeCallsMap:  make(map[string]*models.CallRecord),
        didToCallMap:    make(map[string]string),
        recordingPath:   "/var/spool/asterisk/recordings",
        routingRules:    make(map[string]string),
    }
    
    // Restore active calls
    r.restoreActiveCalls()
    
    // Start cleanup routine
    go r.cleanupRoutine()
    
    return r
}

func (r *Router) ProcessIncomingCall(callID, ani, dnis string) (*models.CallResponse, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    log.Printf("[ROUTER] Processing incoming call - CallID: %s, ANI: %s, DNIS: %s", callID, ani, dnis)
    
    // Determine provider based on routing rules or use round-robin
    providerName := r.selectProvider(dnis)
    
    // Get available DID from selected provider
    did, err := r.providerManager.GetAvailableDID(providerName)
    if err != nil {
        // Try any provider if specific one fails
        did, err = r.providerManager.GetAvailableDID("")
        if err != nil {
            return nil, fmt.Errorf("no available DIDs: %w", err)
        }
    }
    
    // Get provider info for the DID
    var provider *models.Provider
    var providerID int
    
    row := r.db.QueryRow(`
        SELECT p.id, p.name FROM providers p
        JOIN dids d ON d.provider_id = p.id
        WHERE d.did = ?
    `, did)
    
    var actualProviderName string
    if err := row.Scan(&providerID, &actualProviderName); err == nil {
        provider, _ = r.providerManager.GetProvider(actualProviderName)
    }
    
    // Mark DID as in use
    if err := r.markDIDInUse(did, dnis); err != nil {
        return nil, err
    }
    
    // Create call record
    record := &models.CallRecord{
        CallID:       callID,
        OriginalANI:  ani,
        OriginalDNIS: dnis,
        AssignedDID:  did,
        ProviderID:   providerID,
        ProviderName: actualProviderName,
        Status:       models.CallStateActive,
        StartTime:    time.Now(),
        RecordingPath: fmt.Sprintf("%s/%s.wav", r.recordingPath, callID),
    }
    
    // Store in memory
    r.activeCallsMap[callID] = record
    r.didToCallMap[did] = callID
    
    // Store in database
    r.storeCallRecord(record)
    
    // Build response
    response := &models.CallResponse{
        Status:       "success",
        DIDAssigned:  did,
        NextHop:      fmt.Sprintf("trunk-%s", actualProviderName),
        ANIToSend:    dnis,
        DNISToSend:   did,
        ProviderName: actualProviderName,
        TrunkName:    fmt.Sprintf("trunk-%s", actualProviderName),
    }
    
    log.Printf("[ROUTER] Call routed via provider %s - DID: %s", actualProviderName, did)
    
    return response, nil
}

func (r *Router) ProcessReturnCall(ani2, did string) (*models.CallResponse, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    did = strings.TrimSpace(did)
    ani2 = strings.TrimSpace(ani2)
    
    log.Printf("[ROUTER] Processing return call - ANI2: %s, DID: %s", ani2, did)
    
    // Find call by DID
    callID, exists := r.didToCallMap[did]
    if !exists {
        // Try to restore from database
        record, err := r.getCallRecordByDID(did)
        if err != nil {
            return nil, fmt.Errorf("no active call for DID %s", did)
        }
        callID = record.CallID
        r.activeCallsMap[callID] = record
        r.didToCallMap[did] = callID
    }
    
    record := r.activeCallsMap[callID]
    
    // Update status
    r.updateCallStatus(callID, models.CallStateReturned)
    
    response := &models.CallResponse{
        Status:      "success",
        NextHop:     "trunk-s4",
        ANIToSend:   record.OriginalANI,
        DNISToSend:  record.OriginalDNIS,
    }
    
    log.Printf("[ROUTER] Return call processed - Restoring ANI: %s, DNIS: %s", 
        record.OriginalANI, record.OriginalDNIS)
    
    return response, nil
}

func (r *Router) selectProvider(dnis string) string {
    // Implement provider selection logic
    // For now, use round-robin among active providers
    providers := r.providerManager.ListProviders()
    if len(providers) == 0 {
        return ""
    }
    
    // Simple round-robin (can be enhanced with more complex routing)
    return providers[0].Name
}

func (r *Router) markDIDInUse(did, destination string) error {
    _, err := r.db.Exec(`
        UPDATE dids 
        SET in_use = 1, destination = ?, updated_at = NOW()
        WHERE did = ?
    `, destination, did)
    return err
}

func (r *Router) storeCallRecord(record *models.CallRecord) error {
    _, err := r.db.Exec(`
        INSERT INTO call_records 
        (call_id, original_ani, original_dnis, assigned_did, provider_id, 
         provider_name, status, start_time, recording_path)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
        status = VALUES(status),
        provider_id = VALUES(provider_id),
        provider_name = VALUES(provider_name)
    `, record.CallID, record.OriginalANI, record.OriginalDNIS, 
       record.AssignedDID, record.ProviderID, record.ProviderName,
       record.Status, record.StartTime, record.RecordingPath)
    
    return err
}

func (r *Router) updateCallStatus(callID string, status models.CallState) error {
    _, err := r.db.Exec(`
        UPDATE call_records 
        SET status = ?, 
            end_time = CASE WHEN ? IN ('COMPLETED', 'FAILED') THEN NOW() ELSE end_time END,
            duration = CASE WHEN ? IN ('COMPLETED', 'FAILED') THEN TIMESTAMPDIFF(SECOND, start_time, NOW()) ELSE duration END
        WHERE call_id = ?
    `, status, status, status, callID)
    return err
}

func (r *Router) getCallRecordByDID(did string) (*models.CallRecord, error) {
    record := &models.CallRecord{}
    err := r.db.QueryRow(`
        SELECT call_id, original_ani, original_dnis, assigned_did, 
               provider_id, provider_name, status, start_time, recording_path
        FROM call_records
        WHERE assigned_did = ? 
        AND status IN ('ACTIVE', 'FORWARDED', 'RETURNED')
       AND start_time > DATE_SUB(NOW(), INTERVAL 10 MINUTE)
       ORDER BY start_time DESC
       LIMIT 1
   `, did).Scan(
       &record.CallID, &record.OriginalANI, &record.OriginalDNIS,
       &record.AssignedDID, &record.ProviderID, &record.ProviderName,
       &record.Status, &record.StartTime, &record.RecordingPath,
   )
   
   return record, err
}

func (r *Router) restoreActiveCalls() {
   rows, err := r.db.Query(`
       SELECT call_id, original_ani, original_dnis, assigned_did, 
              provider_id, provider_name, status, start_time, recording_path
       FROM call_records
       WHERE status IN ('ACTIVE', 'FORWARDED', 'RETURNED')
       AND start_time > DATE_SUB(NOW(), INTERVAL 10 MINUTE)
   `)
   if err != nil {
       return
   }
   defer rows.Close()
   
   count := 0
   for rows.Next() {
       record := &models.CallRecord{}
       if err := rows.Scan(
           &record.CallID, &record.OriginalANI, &record.OriginalDNIS,
           &record.AssignedDID, &record.ProviderID, &record.ProviderName,
           &record.Status, &record.StartTime, &record.RecordingPath,
       ); err == nil {
           r.activeCallsMap[record.CallID] = record
           r.didToCallMap[record.AssignedDID] = record.CallID
           count++
       }
   }
   
   log.Printf("[ROUTER] Restored %d active calls", count)
}

func (r *Router) cleanupRoutine() {
   ticker := time.NewTicker(30 * time.Second)
   defer ticker.Stop()
   
   for range ticker.C {
       r.cleanupStaleCalls()
   }
}

func (r *Router) cleanupStaleCalls() {
   result, err := r.db.Exec(`
       UPDATE call_records 
       SET status = 'FAILED', end_time = NOW()
       WHERE status IN ('ACTIVE', 'FORWARDED')
       AND start_time < DATE_SUB(NOW(), INTERVAL 10 MINUTE)
   `)
   
   if err == nil {
       rows, _ := result.RowsAffected()
       if rows > 0 {
           log.Printf("[ROUTER] Cleaned up %d stale calls", rows)
           
           // Release DIDs
           r.db.Exec(`
               UPDATE dids d
               INNER JOIN call_records cr ON d.did = cr.assigned_did
               SET d.in_use = 0, d.destination = NULL
               WHERE cr.status = 'FAILED'
               AND cr.end_time > DATE_SUB(NOW(), INTERVAL 1 MINUTE)
           `)
       }
   }
}

func (r *Router) GetStatistics() map[string]interface{} {
   r.mu.RLock()
   activeCalls := len(r.activeCallsMap)
   r.mu.RUnlock()
   
   stats := map[string]interface{}{
       "active_calls": activeCalls,
       "providers":    make([]map[string]interface{}, 0),
   }
   
   // Get provider statistics
   for _, p := range r.providerManager.ListProviders() {
       if pStats, err := r.providerManager.GetProviderStats(p.Name); err == nil {
           stats["providers"] = append(stats["providers"].([]map[string]interface{}), pStats)
       }
   }
   
   // Get overall statistics
   var totalDIDs, usedDIDs, totalCalls, completedCalls int
   
   r.db.QueryRow("SELECT COUNT(*), SUM(CASE WHEN in_use = 1 THEN 1 ELSE 0 END) FROM dids").Scan(&totalDIDs, &usedDIDs)
   r.db.QueryRow(`
       SELECT COUNT(*), SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END)
       FROM call_records WHERE DATE(start_time) = CURDATE()
   `).Scan(&totalCalls, &completedCalls)
   
   stats["total_dids"] = totalDIDs
   stats["used_dids"] = usedDIDs
   stats["available_dids"] = totalDIDs - usedDIDs
   stats["calls_today"] = totalCalls
   stats["completed_calls"] = completedCalls
   stats["timestamp"] = time.Now().Format(time.RFC3339)
   
   return stats
}
