package provider

import (
    "fmt"
    "os"
    "os/exec"  // Add this import
    "path/filepath"
    "strings"
    "text/template"
    
    "github.com/router-production/internal/models"
)

type AsteriskConfigGenerator struct {
    configPath string
    templates  map[string]*template.Template
}

func NewAsteriskConfigGenerator() *AsteriskConfigGenerator {
    g := &AsteriskConfigGenerator{
        configPath: "/etc/asterisk",
        templates:  make(map[string]*template.Template),
    }
    
    g.loadTemplates()
    return g
}

func (g *AsteriskConfigGenerator) loadTemplates() {
    // PJSIP endpoint template
    pjsipTemplate := `
;========== Provider: {{.Name}} ==========
[trunk-{{.Name}}]
type=endpoint
transport=transport-udp
context=from-provider-{{.Name}}
disallow=all
{{range .Codecs}}allow={{.}}
{{end}}auth=trunk-{{.Name}}-auth
aors=trunk-{{.Name}}-aor
{{if .Username}}outbound_auth=trunk-{{.Name}}-auth{{end}}
direct_media=no
force_rport=yes
rewrite_contact=yes
rtp_symmetric=yes
{{if .MaxChannels}}max_audio_streams={{.MaxChannels}}{{end}}

[trunk-{{.Name}}-aor]
type=aor
contact=sip:{{.Host}}:{{.Port}}
qualify_frequency=30
max_contacts=1

[trunk-{{.Name}}-identify]
type=identify
endpoint=trunk-{{.Name}}
match={{.Host}}

{{if .Username}}
[trunk-{{.Name}}-auth]
type=auth
auth_type=userpass
username={{.Username}}
password={{.Password}}
{{if .Realm}}realm={{.Realm}}{{end}}
{{end}}
`
    
    g.templates["pjsip"] = template.Must(template.New("pjsip").Parse(pjsipTemplate))
    
    // Extensions context template
    extensionsTemplate := `
[from-provider-{{.Name}}]
; Incoming calls from provider {{.Name}}
exten => _X.,1,NoOp(Call from provider {{.Name}})
exten => _X.,n,Goto(from-s3,${EXTEN},1)
`
    
    g.templates["extensions"] = template.Must(template.New("extensions").Parse(extensionsTemplate))
}

func (g *AsteriskConfigGenerator) GenerateProviderConfig(p *models.Provider) error {
    // Generate PJSIP config
    pjsipFile := filepath.Join(g.configPath, fmt.Sprintf("pjsip_provider_%s.conf", p.Name))
    if err := g.generateConfig(pjsipFile, "pjsip", p); err != nil {
        return err
    }
    
    // Generate extensions config
    extFile := filepath.Join(g.configPath, fmt.Sprintf("extensions_provider_%s.conf", p.Name))
    if err := g.generateConfig(extFile, "extensions", p); err != nil {
        return err
    }
    
    // Update main configs to include provider configs
    g.updateMainConfigs(p.Name)
    
    // Reload Asterisk
    if err := g.reloadAsterisk(); err != nil {
        return fmt.Errorf("failed to reload asterisk: %w", err)
    }
    
    return nil
}

func (g *AsteriskConfigGenerator) generateConfig(filename, templateName string, p *models.Provider) error {
    file, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer file.Close()
    
    return g.templates[templateName].Execute(file, p)
}

func (g *AsteriskConfigGenerator) updateMainConfigs(providerName string) {
    // Add include to pjsip.conf if not exists
    pjsipInclude := fmt.Sprintf("#include pjsip_provider_%s.conf", providerName)
    g.addIncludeIfNotExists("/etc/asterisk/pjsip.conf", pjsipInclude)
    
    // Add include to extensions.conf if not exists
    extInclude := fmt.Sprintf("#include extensions_provider_%s.conf", providerName)
    g.addIncludeIfNotExists("/etc/asterisk/extensions.conf", extInclude)
}

func (g *AsteriskConfigGenerator) addIncludeIfNotExists(filename, include string) {
    content, err := os.ReadFile(filename)
    if err != nil {
        return
    }
    
    if !strings.Contains(string(content), include) {
        file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
        if err != nil {
            return
        }
        defer file.Close()
        
        file.WriteString("\n" + include + "\n")
    }
}

// Fixed reloadAsterisk function
func (g *AsteriskConfigGenerator) reloadAsterisk() error {
    cmd := exec.Command("asterisk", "-rx", "core reload")
    return cmd.Run()
}
