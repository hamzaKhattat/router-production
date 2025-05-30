package main

import (
   "fmt"
   "log"
   "os"
   
   "strings"
   "github.com/router-production/internal/models"
   
   "github.com/spf13/cobra"
   "github.com/router-production/internal/api"
   "github.com/router-production/internal/database"
   "github.com/router-production/internal/provider"
   "github.com/router-production/internal/router"
)

var (
   dbHost string
   dbPort int
   dbUser string
   dbPass string
   dbName string
)

func main() {
   var rootCmd = &cobra.Command{
       Use:   "router",
       Short: "Production S2 Router with Provider Management",
   }
   
   // Global flags
   rootCmd.PersistentFlags().StringVar(&dbHost, "db-host", "localhost", "Database host")
   rootCmd.PersistentFlags().IntVar(&dbPort, "db-port", 3306, "Database port")
   rootCmd.PersistentFlags().StringVar(&dbUser, "db-user", "root", "Database user")
   rootCmd.PersistentFlags().StringVar(&dbPass, "db-pass", "temppass", "Database password")
   rootCmd.PersistentFlags().StringVar(&dbName, "db-name", "call_routing", "Database name")
   
   // Add commands
   rootCmd.AddCommand(serverCmd())
   rootCmd.AddCommand(providerCmd())
   rootCmd.AddCommand(didCmd())
   rootCmd.AddCommand(statsCmd())
   
   if err := rootCmd.Execute(); err != nil {
       fmt.Fprintln(os.Stderr, err)
       os.Exit(1)
   }
}

func getDB() (*database.DB, error) {
   dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
       dbUser, dbPass, dbHost, dbPort, dbName)
   
   db, err := database.NewDB(dsn)
   if err != nil {
       return nil, err
   }
   
   // Create tables
   if err := db.CreateTables(); err != nil {
       return nil, err
   }
   
   return db, nil
}

func serverCmd() *cobra.Command {
   var port int
   
   cmd := &cobra.Command{
       Use:   "server",
       Short: "Start the router server",
       RunE: func(cmd *cobra.Command, args []string) error {
           db, err := getDB()
           if err != nil {
               return err
           }
           
           // Initialize components
           pm := provider.NewManager(db)
           r := router.NewRouter(db, pm)
           
           // Start API server
           server := api.NewServer(r, pm, port)
           
           log.Printf("Starting router server on port %d", port)
           return server.Start()
       },
   }
   
   cmd.Flags().IntVarP(&port, "port", "p", 8001, "Server port")
   
   return cmd
}

func providerCmd() *cobra.Command {
   cmd := &cobra.Command{
       Use:   "provider",
       Short: "Manage providers",
   }
   
   // Add provider
   addCmd := &cobra.Command{
       Use:   "add",
       Short: "Add a new provider",
       RunE: func(cmd *cobra.Command, args []string) error {
           name, _ := cmd.Flags().GetString("name")
           host, _ := cmd.Flags().GetString("host")
           port, _ := cmd.Flags().GetInt("port")
           username, _ := cmd.Flags().GetString("username")
           password, _ := cmd.Flags().GetString("password")
           realm, _ := cmd.Flags().GetString("realm")
           codecs, _ := cmd.Flags().GetStringSlice("codecs")
           maxChannels, _ := cmd.Flags().GetInt("max-channels")
           country, _ := cmd.Flags().GetString("country")
           
           db, err := getDB()
           if err != nil {
               return err
           }
           
           pm := provider.NewManager(db)
           
           p := &models.Provider{
               Name:        name,
               Host:        host,
               Port:        port,
               Username:    username,
               Password:    password,
               Realm:       realm,
               Codecs:      codecs,
               MaxChannels: maxChannels,
               Country:     country,
               Active:      true,
           }
           
           if err := pm.AddProvider(p); err != nil {
               return err
           }
           
           fmt.Printf("Provider %s added successfully\n", name)
           return nil
       },
   }
   
   addCmd.Flags().String("name", "", "Provider name (required)")
   addCmd.Flags().String("host", "", "Provider host/IP (required)")
   addCmd.Flags().Int("port", 5060, "Provider port")
   addCmd.Flags().String("username", "", "SIP username")
   addCmd.Flags().String("password", "", "SIP password")
   addCmd.Flags().String("realm", "", "SIP realm")
   addCmd.Flags().StringSlice("codecs", []string{"ulaw", "alaw"}, "Supported codecs")
   addCmd.Flags().Int("max-channels", 100, "Maximum concurrent channels")
   addCmd.Flags().String("country", "", "Provider country")
   addCmd.MarkFlagRequired("name")
   addCmd.MarkFlagRequired("host")
   
   // List providers
   listCmd := &cobra.Command{
       Use:   "list",
       Short: "List all providers",
       RunE: func(cmd *cobra.Command, args []string) error {
           db, err := getDB()
           if err != nil {
               return err
           }
           
           pm := provider.NewManager(db)
           providers := pm.ListProviders()
           
           fmt.Printf("%-15s %-20s %-10s %-10s %-10s\n", "NAME", "HOST", "PORT", "COUNTRY", "ACTIVE")
           fmt.Println(strings.Repeat("-", 70))
           
           for _, p := range providers {
               fmt.Printf("%-15s %-20s %-10d %-10s %-10v\n", 
                   p.Name, p.Host, p.Port, p.Country, p.Active)
           }
           
           return nil
       },
   }
   
   cmd.AddCommand(addCmd)
   cmd.AddCommand(listCmd)
   
   return cmd
}

func didCmd() *cobra.Command {
   cmd := &cobra.Command{
       Use:   "did",
       Short: "Manage DIDs",
   }
   
   // Add DIDs
   addCmd := &cobra.Command{
       Use:   "add",
       Short: "Add DIDs to a provider",
       RunE: func(cmd *cobra.Command, args []string) error {
           provider, _ := cmd.Flags().GetString("provider")
           dids, _ := cmd.Flags().GetStringSlice("dids")
           file, _ := cmd.Flags().GetString("file")
           country, _ := cmd.Flags().GetString("country")
           
           if file != "" {
               // Load DIDs from file
               content, err := os.ReadFile(file)
               if err != nil {
                   return err
               }
               dids = strings.Split(string(content), "\n")
           }
           
           // Clean DIDs
           cleanDIDs := make([]string, 0, len(dids))
           for _, did := range dids {
               did = strings.TrimSpace(did)
               if did != "" {
                   cleanDIDs = append(cleanDIDs, did)
               }
           }
           
           db, err := getDB()
           if err != nil {
               return err
           }
           
           pm := provider.NewManager(db)
           
           if err := pm.AddDIDs(provider, cleanDIDs, country); err != nil {
               return err
           }
           
           fmt.Printf("Added %d DIDs to provider %s\n", len(cleanDIDs), provider)
           return nil
       },
   }
   
   addCmd.Flags().String("provider", "", "Provider name (required)")
   addCmd.Flags().StringSlice("dids", []string{}, "List of DIDs")
   addCmd.Flags().String("file", "", "File containing DIDs (one per line)")
   addCmd.Flags().String("country", "", "Country for these DIDs")
   addCmd.MarkFlagRequired("provider")
   
   cmd.AddCommand(addCmd)
   
   return cmd
}

func statsCmd() *cobra.Command {
   return &cobra.Command{
       Use:   "stats",
       Short: "Show router statistics",
       RunE: func(cmd *cobra.Command, args []string) error {
           db, err := getDB()
           if err != nil {
               return err
           }
           
           pm := provider.NewManager(db)
           r := router.NewRouter(db, pm)
           
           stats := r.GetStatistics()
           
           // Print statistics
           fmt.Printf("\n=== ROUTER STATISTICS ===\n")
           fmt.Printf("Timestamp: %s\n", stats["timestamp"])
           fmt.Printf("Active Calls: %d\n", stats["active_calls"])
           fmt.Printf("Total DIDs: %d\n", stats["total_dids"])
           fmt.Printf("Used DIDs: %d\n", stats["used_dids"])
           fmt.Printf("Available DIDs: %d\n", stats["available_dids"])
           fmt.Printf("Calls Today: %d\n", stats["calls_today"])
           fmt.Printf("Completed Calls: %d\n\n", stats["completed_calls"])
           
           // Provider statistics
           if providers, ok := stats["providers"].([]map[string]interface{}); ok && len(providers) > 0 {
               fmt.Printf("=== PROVIDER STATISTICS ===\n")
               for _, pStats := range providers {
                   if p, ok := pStats["provider"].(*models.Provider); ok {
                       fmt.Printf("\nProvider: %s\n", p.Name)
                       fmt.Printf("  Total DIDs: %d\n", pStats["total_dids"])
                       fmt.Printf("  Used DIDs: %d\n", pStats["used_dids"])
                       fmt.Printf("  Available DIDs: %d\n", pStats["available_dids"])
                       fmt.Printf("  Calls Today: %d\n", pStats["calls_today"])
                       fmt.Printf("  Active Calls: %d\n", pStats["active_calls"])
                   }
               }
           }
           
           return nil
       },
   }
}

// Add missing imports

