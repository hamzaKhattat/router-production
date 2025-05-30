package models

import (
    "time"
)

type Provider struct {
    ID          int       `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`
    Host        string    `json:"host" db:"host"`
    Port        int       `json:"port" db:"port"`
    Username    string    `json:"username" db:"username"`
    Password    string    `json:"password" db:"password"`
    Realm       string    `json:"realm" db:"realm"`
    Transport   string    `json:"transport" db:"transport"`
    Codecs      []string  `json:"codecs" db:"codecs"`
    MaxChannels int       `json:"max_channels" db:"max_channels"`
    Active      bool      `json:"active" db:"active"`
    Country     string    `json:"country" db:"country"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type DID struct {
    ID           int       `json:"id" db:"id"`
    DID          string    `json:"did" db:"did"`
    ProviderID   int       `json:"provider_id" db:"provider_id"`
    ProviderName string    `json:"provider_name" db:"provider_name"`
    InUse        bool      `json:"in_use" db:"in_use"`
    Destination  string    `json:"destination" db:"destination"`
    Country      string    `json:"country" db:"country"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
    UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type CallState string

const (
    CallStateActive     CallState = "ACTIVE"
    CallStateForwarded  CallState = "FORWARDED"
    CallStateReturned   CallState = "RETURNED"
    CallStateCompleted  CallState = "COMPLETED"
    CallStateFailed     CallState = "FAILED"
)

type CallRecord struct {
    ID            int64      `json:"id" db:"id"`
    CallID        string     `json:"call_id" db:"call_id"`
    OriginalANI   string     `json:"original_ani" db:"original_ani"`
    OriginalDNIS  string     `json:"original_dnis" db:"original_dnis"`
    AssignedDID   string     `json:"assigned_did" db:"assigned_did"`
    ProviderID    int        `json:"provider_id" db:"provider_id"`
    ProviderName  string     `json:"provider_name" db:"provider_name"`
    Status        CallState  `json:"status" db:"status"`
    StartTime     time.Time  `json:"start_time" db:"start_time"`
    EndTime       *time.Time `json:"end_time" db:"end_time"`
    Duration      int        `json:"duration" db:"duration"`
    RecordingPath string     `json:"recording_path" db:"recording_path"`
}

type CallResponse struct {
    Status       string `json:"status"`
    DIDAssigned  string `json:"did_assigned"`
    NextHop      string `json:"next_hop"`
    ANIToSend    string `json:"ani_to_send"`
    DNISToSend   string `json:"dnis_to_send"`
    ProviderName string `json:"provider_name"`
    TrunkName    string `json:"trunk_name"`
}
