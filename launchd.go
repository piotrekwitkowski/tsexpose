package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

const plistLabel = "com.tsexpose"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>-local-port</string>
        <string>{{.LocalPort}}</string>
        <string>-ts-port</string>
        <string>{{.TsPort}}</string>
        <string>-hostname</string>
        <string>{{.Hostname}}</string>
{{- if .StateDir}}
        <string>-state-dir</string>
        <string>{{.StateDir}}</string>
{{- end}}
{{- if .Ephemeral}}
        <string>-ephemeral</string>
{{- end}}
{{- if .TsLogs}}
        <string>-ts-logs</string>
{{- end}}
    </array>
{{- if .AuthKey}}

    <key>EnvironmentVariables</key>
    <dict>
        <key>TS_AUTHKEY</key>
        <string>{{.AuthKey}}</string>
    </dict>
{{- end}}

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>/tmp/tsexpose-{{.Hostname}}.log</string>

    <key>StandardErrorPath</key>
    <string>/tmp/tsexpose-{{.Hostname}}.log</string>
</dict>
</plist>
`

type plistData struct {
	Label      string
	BinaryPath string
	LocalPort  int
	TsPort     int
	Hostname   string
	StateDir   string
	AuthKey    string
	Ephemeral  bool
	TsLogs     bool
}

func plistPath(hostname string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	label := fmt.Sprintf("%s.%s", plistLabel, hostname)
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func installLaunchd(localPort, tsPort int, hostname, stateDir, authKey string, ephemeral, tsLogs bool) {
	if runtime.GOOS != "darwin" {
		log.Fatal("--install is only supported on macOS")
	}

	binPath, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to resolve binary path: %v", err)
	}
	binPath, err = filepath.EvalSymlinks(binPath)
	if err != nil {
		log.Fatalf("failed to resolve binary symlinks: %v", err)
	}

	data := plistData{
		Label:      fmt.Sprintf("%s.%s", plistLabel, hostname),
		BinaryPath: binPath,
		LocalPort:  localPort,
		TsPort:     tsPort,
		Hostname:   hostname,
		StateDir:   stateDir,
		AuthKey:    authKey,
		Ephemeral:  ephemeral,
		TsLogs:     tsLogs,
	}

	path := plistPath(hostname)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Fatalf("failed to create LaunchAgents directory: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("failed to create plist: %v", err)
	}
	defer f.Close()

	tmpl := template.Must(template.New("plist").Parse(plistTemplate))
	if err := tmpl.Execute(f, data); err != nil {
		log.Fatalf("failed to write plist: %v", err)
	}

	cmd := exec.Command("launchctl", "load", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("launchctl load failed: %v", err)
	}

	log.Printf("installed: %s", path)
	log.Printf("tsexpose will start automatically at login")
	log.Printf("logs: /tmp/tsexpose-%s.log", hostname)
}

func uninstallLaunchd(hostname string) {
	if runtime.GOOS != "darwin" {
		log.Fatal("--uninstall is only supported on macOS")
	}

	path := plistPath(hostname)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Fatalf("no installation found at %s", path)
	}

	cmd := exec.Command("launchctl", "unload", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("warning: launchctl unload failed: %v", err)
	}

	if err := os.Remove(path); err != nil {
		log.Fatalf("failed to remove plist: %v", err)
	}

	log.Printf("uninstalled: %s", path)
}
