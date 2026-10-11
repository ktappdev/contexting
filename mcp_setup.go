package contexting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

type mcpSetupClient struct {
	name       string
	configPath string
	format     string
}

type mcpSetupChange struct {
	client mcpSetupClient
	data   []byte
	mode   os.FileMode
	status string
}

type mcpServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func newMCPSetupCommand(parent *cobra.Command) *cobra.Command {
	var selectedClients []string
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "setup [path]",
		Short: "Connect local agent clients, then start the MCP server",
		Long: `Detect and configure Claude Code, Codex, and Pi for this project.

The command previews project-local config changes, asks before writing, and then
starts the same local stdio MCP server as "ctxt mcp". Use --client to select
clients explicitly or --yes to confirm the preview without an interactive prompt.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectPath := "."
			if len(args) == 1 {
				projectPath = args[0]
			}
			projectRoot, err := filepath.Abs(projectPath)
			if err != nil {
				return fmt.Errorf("resolve project path: %w", err)
			}
			rootInfo, err := os.Stat(projectRoot)
			if err != nil {
				return fmt.Errorf("inspect project path: %w", err)
			}
			if !rootInfo.IsDir() {
				return fmt.Errorf("project path is not a directory: %s", projectRoot)
			}
			indexPath := resolveProjectPath(projectRoot, ".ctxt/ctx_index.json")
			if _, err := os.Stat(indexPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("no index found at %s; run 'ctxt init %s' first", indexPath, projectRoot)
				}
				return fmt.Errorf("check index at %s: %w", indexPath, err)
			}

			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve ctxt executable: %w", err)
			}
			executable, err = filepath.Abs(executable)
			if err != nil {
				return fmt.Errorf("resolve ctxt executable path: %w", err)
			}

			clients, err := selectMCPSetupClients(projectRoot, selectedClients)
			if err != nil {
				return err
			}
			if len(clients) == 0 {
				return errors.New("no supported MCP clients detected; install Claude Code, Codex, or Pi, or select one with --client")
			}

			changes := make([]mcpSetupChange, 0, len(clients))
			for _, client := range clients {
				change, err := planMCPSetupChange(client, projectRoot, executable)
				if err != nil {
					return err
				}
				changes = append(changes, change)
			}

			fmt.Printf("MCP setup for %s\n", projectRoot)
			for _, change := range changes {
				fmt.Printf("  %s: %s (%s)\n", change.client.name, change.client.configPath, change.status)
				fmt.Printf("    command: %q\n    args: [\"mcp\", %q]\n", executable, projectRoot)
			}
			if !assumeYes {
				if !isInteractiveTerminal() {
					return errors.New("review the preview above and rerun with --yes to write these project configs")
				}
				ok, err := askYesNo("Write these config changes and start the MCP server? [y/N]: ", false)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("MCP setup canceled")
				}
			}

			for _, change := range changes {
				if change.status == "already configured" {
					continue
				}
				if err := writeFileAtomic(change.client.configPath, change.data, change.mode); err != nil {
					return fmt.Errorf("write %s MCP config: %w", change.client.name, err)
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "MCP clients configured. Starting ctxt MCP server; press Ctrl+C to stop.")

			parent.SetContext(cmd.Context())
			return parent.RunE(parent, []string{projectRoot})
		},
	}
	cmd.Flags().StringSliceVar(&selectedClients, "client", nil, "Clients to configure: claude-code, codex, pi (default: installed clients)")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "Write the displayed config changes without prompting")
	return cmd
}

func selectMCPSetupClients(projectRoot string, requested []string) ([]mcpSetupClient, error) {
	definitions := map[string]mcpSetupClient{
		"claude-code": {name: "Claude Code", configPath: filepath.Join(projectRoot, ".mcp.json"), format: "json"},
		"codex":       {name: "Codex", configPath: filepath.Join(projectRoot, ".codex", "config.toml"), format: "toml"},
		"pi":          {name: "Pi", configPath: filepath.Join(projectRoot, ".pi", "mcp.json"), format: "json"},
	}
	if len(requested) == 0 {
		for _, name := range []string{"claude-code", "codex", "pi"} {
			binary := name
			if name == "claude-code" {
				binary = "claude"
			}
			if _, err := exec.LookPath(binary); err == nil {
				requested = append(requested, name)
			}
		}
	}

	selected := make([]mcpSetupClient, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, name := range requested {
		name = strings.ToLower(strings.TrimSpace(name))
		client, ok := definitions[name]
		if !ok {
			return nil, fmt.Errorf("unknown MCP client %q; choose claude-code, codex, or pi", name)
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		selected = append(selected, client)
	}
	return selected, nil
}

func planMCPSetupChange(client mcpSetupClient, projectRoot, executable string) (mcpSetupChange, error) {
	change := mcpSetupChange{client: client, mode: 0o644, status: "create"}
	info, err := os.Stat(client.configPath)
	if err == nil {
		change.mode = info.Mode().Perm()
		change.status = "update"
	} else if !errors.Is(err, os.ErrNotExist) {
		return mcpSetupChange{}, fmt.Errorf("inspect %s config: %w", client.name, err)
	}

	config := mcpServerConfig{Command: executable, Args: []string{"mcp", projectRoot}}
	switch client.format {
	case "json":
		data, status, err := mergeJSONMCPConfig(client.configPath, config)
		if err != nil {
			return mcpSetupChange{}, fmt.Errorf("prepare %s config: %w", client.name, err)
		}
		change.data = data
		if status == "already configured" {
			change.status = status
		} else if change.status == "create" {
			change.status = status
		}
	case "toml":
		data, status, err := mergeCodexMCPConfig(client.configPath, config)
		if err != nil {
			return mcpSetupChange{}, fmt.Errorf("prepare %s config: %w", client.name, err)
		}
		change.data = data
		if status == "already configured" {
			change.status = status
		} else if change.status == "create" {
			change.status = status
		}
	default:
		return mcpSetupChange{}, fmt.Errorf("unsupported MCP config format %q", client.format)
	}
	return change, nil
}

func mergeJSONMCPConfig(path string, config mcpServerConfig) ([]byte, string, error) {
	root := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, "", fmt.Errorf("decode JSON: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}

	servers := make(map[string]json.RawMessage)
	if raw, ok := root["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return nil, "", fmt.Errorf("decode mcpServers: %w", err)
		}
	}
	serverData, err := json.Marshal(config)
	if err != nil {
		return nil, "", err
	}
	if existing, ok := servers["ctxt"]; ok {
		var existingConfig mcpServerConfig
		if err := json.Unmarshal(existing, &existingConfig); err != nil {
			return nil, "", fmt.Errorf("decode existing ctxt server: %w", err)
		}
		if existingConfig.Command == config.Command && equalStrings(existingConfig.Args, config.Args) {
			return nil, "already configured", nil
		}
	}
	servers["ctxt"] = serverData
	encodedServers, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return nil, "", err
	}
	root["mcpServers"] = encodedServers
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return append(data, '\n'), "update", nil
}

func mergeCodexMCPConfig(path string, config mcpServerConfig) ([]byte, string, error) {
	var existingData []byte
	if data, err := os.ReadFile(path); err == nil {
		existingData = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}

	var parsed map[string]any
	if _, err := toml.Decode(string(existingData), &parsed); err != nil {
		return nil, "", fmt.Errorf("decode TOML: %w", err)
	}
	if serversValue, ok := parsed["mcp_servers"]; ok {
		servers, ok := serversValue.(map[string]any)
		if !ok {
			return nil, "", errors.New("mcp_servers must be a TOML table")
		}
		if existingValue, ok := servers["ctxt"]; ok {
			entry, ok := existingValue.(map[string]any)
			if !ok {
				return nil, "", errors.New("mcp_servers.ctxt must be a TOML table")
			}
			existingCommand, _ := entry["command"].(string)
			existingArgs, _ := entry["args"].([]any)
			args := make([]string, 0, len(existingArgs))
			for _, arg := range existingArgs {
				value, ok := arg.(string)
				if !ok {
					return nil, "", errors.New("mcp_servers.ctxt.args must contain strings")
				}
				args = append(args, value)
			}
			if existingCommand == config.Command && equalStrings(args, config.Args) {
				return nil, "already configured", nil
			}
			return nil, "", errors.New("mcp_servers.ctxt already exists with different settings; update or remove it before running setup")
		}
	}

	var output bytes.Buffer
	if len(existingData) > 0 {
		output.Write(existingData)
		if existingData[len(existingData)-1] != '\n' {
			output.WriteByte('\n')
		}
		output.WriteByte('\n')
	}
	output.WriteString("[mcp_servers.ctxt]\n")
	output.WriteString("command = ")
	output.WriteString(strconv.Quote(config.Command))
	output.WriteString("\nargs = [")
	for i, arg := range config.Args {
		if i > 0 {
			output.WriteString(", ")
		}
		output.WriteString(strconv.Quote(arg))
	}
	output.WriteString("]\n")
	return output.Bytes(), "update", nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
