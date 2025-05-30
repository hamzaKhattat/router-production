package database

import (
    "database/sql"
    "fmt"
    "log"
    "time"
    
    _ "github.com/go-sql-driver/mysql"
)

type DB struct {
    *sql.DB
}

func NewDB(dsn string) (*DB, error) {
	log.Printf("hello world")
    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, err
    }
    
    if err := db.Ping(); err != nil {
        return nil, err
    }
    
    // Set connection pool settings
    db.SetMaxOpenConns(50)
    db.SetMaxIdleConns(10)
    db.SetConnMaxLifetime(5 * time.Minute)
    
    return &DB{db}, nil
}

func (db *DB) CreateTables() error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS providers (
            id INT AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(100) UNIQUE NOT NULL,
            host VARCHAR(255) NOT NULL,
            port INT DEFAULT 5060,
            username VARCHAR(100),
            password VARCHAR(255),
            realm VARCHAR(255),
            transport VARCHAR(50) DEFAULT 'udp',
            codecs JSON,
            max_channels INT DEFAULT 100,
            active BOOLEAN DEFAULT TRUE,
            country VARCHAR(50),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            INDEX idx_name (name),
            INDEX idx_active (active)
        )`,
        
        `CREATE TABLE IF NOT EXISTS dids (
            id INT AUTO_INCREMENT PRIMARY KEY,
            did VARCHAR(50) NOT NULL,
            provider_id INT NOT NULL,
            provider_name VARCHAR(100),
            in_use BOOLEAN DEFAULT FALSE,
            destination VARCHAR(50),
            country VARCHAR(50),
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            INDEX idx_did (did),
            INDEX idx_provider (provider_id),
            INDEX idx_in_use (in_use),
            FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
        )`,
        
        `CREATE TABLE IF NOT EXISTS call_records (
            id BIGINT AUTO_INCREMENT PRIMARY KEY,
            call_id VARCHAR(100) UNIQUE NOT NULL,
            original_ani VARCHAR(50),
            original_dnis VARCHAR(50),
            assigned_did VARCHAR(50),
            provider_id INT,
            provider_name VARCHAR(100),
            status VARCHAR(50),
            start_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            end_time TIMESTAMP NULL,
            duration INT DEFAULT 0,
            recording_path VARCHAR(255),
            INDEX idx_call_id (call_id),
            INDEX idx_did (assigned_did),
            INDEX idx_provider (provider_id),
            INDEX idx_status (status),
            INDEX idx_start_time (start_time)
        )`,
        
        `CREATE TABLE IF NOT EXISTS provider_configs (
            id INT AUTO_INCREMENT PRIMARY KEY,
            provider_id INT NOT NULL,
            config_type VARCHAR(50),
            config_data JSON,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
        )`,
    }
    
    for _, query := range queries {
        if _, err := db.Exec(query); err != nil {
            return fmt.Errorf("failed to create table: %w", err)
        }
    }
    
    return nil
}
