package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gmssh/gmssh/internal/api"
	"github.com/gmssh/gmssh/internal/cli"
	"github.com/gmssh/gmssh/pkg/types"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// 创建 CLI 实例
	c, err := cli.NewCLI()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "upload":
		uploadCmd := flag.NewFlagSet("upload", flag.ExitOnError)
		source := uploadCmd.String("source", "", "Source file path")
		target := uploadCmd.String("target", "", "Target host:path")
		via := uploadCmd.String("via", "", "Comma-separated list of intermediate hops")
		uploadCmd.Parse(os.Args[2:])

		if *source == "" || *target == "" {
			fmt.Fprintln(os.Stderr, "Error: source and target are required")
			uploadCmd.Usage()
			os.Exit(1)
		}

		var viaList []string
		if *via != "" {
			viaList = strings.Split(*via, ",")
		}

		if err := c.UploadCommand(*source, *target, viaList); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "proxy":
		proxyCmd := flag.NewFlagSet("proxy", flag.ExitOnError)
		local := proxyCmd.String("local", ":0", "Local listen address")
		remoteHost := proxyCmd.String("remote-host", "", "Remote target host")
		remotePort := proxyCmd.Int("remote-port", 0, "Remote target port")
		via := proxyCmd.String("via", "", "Comma-separated list of intermediate hops")
		proxyCmd.Parse(os.Args[2:])

		if *remoteHost == "" || *remotePort == 0 {
			fmt.Fprintln(os.Stderr, "Error: remote-host and remote-port are required")
			proxyCmd.Usage()
			os.Exit(1)
		}

		var viaList []string
		if *via != "" {
			viaList = strings.Split(*via, ",")
		}

		if err := c.ProxyCommand(*local, *remoteHost, *remotePort, viaList); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "probe":
		probeCmd := flag.NewFlagSet("probe", flag.ExitOnError)
		target := probeCmd.String("target", "", "Target host to probe")
		via := probeCmd.String("via", "", "Comma-separated list of intermediate hops")
		probeCmd.Parse(os.Args[2:])

		if *target == "" {
			fmt.Fprintln(os.Stderr, "Error: target is required")
			probeCmd.Usage()
			os.Exit(1)
		}

		var viaList []string
		if *via != "" {
			viaList = strings.Split(*via, ",")
		}

		if err := c.ProbeCommand(*target, viaList); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "status":
		if err := c.StatusCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "server":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: server subcommand required (add, list, delete)")
			os.Exit(1)
		}

		subCommand := os.Args[2]
		switch subCommand {
		case "list":
			if err := c.ServerListCommand(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

		case "add":
			addCmd := flag.NewFlagSet("server add", flag.ExitOnError)
			name := addCmd.String("name", "", "Server name")
			host := addCmd.String("host", "", "Server host")
			port := addCmd.Int("port", 22, "Server port")
			user := addCmd.String("user", "", "Username")
			authType := addCmd.String("auth", "key", "Auth type: key or password")
			keyPath := addCmd.String("key-path", "", "SSH key path (for key auth)")
			password := addCmd.String("password", "", "Password (for password auth)")
			addCmd.Parse(os.Args[3:])

			if *name == "" || *host == "" || *user == "" {
				fmt.Fprintln(os.Stderr, "Error: name, host, and user are required")
				addCmd.Usage()
				os.Exit(1)
			}

			var auth types.AuthMethod
			switch *authType {
			case "key":
				auth = types.AuthKey
				if *keyPath == "" {
					*keyPath = "~/.ssh/id_rsa"
				}
			case "password":
				auth = types.AuthPassword
			default:
				fmt.Fprintf(os.Stderr, "Error: invalid auth type '%s'\n", *authType)
				os.Exit(1)
			}

			hop := &types.Hop{
				Name:     *name,
				Host:     *host,
				Port:     *port,
				User:     *user,
				AuthType: auth,
				KeyPath:  *keyPath,
				Password: *password,
			}

			if err := c.ServerAddCommand(hop); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

		case "delete":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: server name required")
				os.Exit(1)
			}
			name := os.Args[3]
			if err := c.ServerDeleteCommand(name); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

		default:
			fmt.Fprintf(os.Stderr, "Unknown server subcommand: %s\n", subCommand)
			os.Exit(1)
		}

	case "web":
		webCmd := flag.NewFlagSet("web", flag.ExitOnError)
		local := webCmd.Bool("local", false, "Run in local mode (localhost only)")
		bind := webCmd.String("bind", "0.0.0.0:18081", "Bind address")
		webCmd.Parse(os.Args[2:])

		addr := *bind
		if *local {
			addr = "127.0.0.1:8080"
		}

		server, err := api.NewServer()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Starting web UI at http://%s\n", addr)
		if err := server.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "portal":
		portalCmd := &cli.PortalCommand{}
		f := flag.NewFlagSet("portal", flag.ExitOnError)
		portalCmd.SetFlags(f)
		f.Parse(os.Args[2:])

		exitCode := portalCmd.Run(f.Args())
		os.Exit(exitCode)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("GMSSH - High-performance SSH bastion tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gmssh <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  upload    Upload file to remote server")
	fmt.Println("            --source <path>       Source file path")
	fmt.Println("            --target <host:path>  Target host and path")
	fmt.Println("            --via <hops>          Comma-separated intermediate hops (optional)")
	fmt.Println()
	fmt.Println("  proxy     Create port forward to internal server")
	fmt.Println("            --local <addr>        Local listen address (default :0)")
	fmt.Println("            --remote-host <host>  Remote target host")
	fmt.Println("            --remote-port <port>  Remote target port")
	fmt.Println("            --via <hops>          Comma-separated intermediate hops")
	fmt.Println()
	fmt.Println("  probe     Probe network latency")
	fmt.Println("            --target <host>       Target host to probe")
	fmt.Println("            --via <hops>          Compare with alternative path")
	fmt.Println()
	fmt.Println("  status    Show configuration status")
	fmt.Println()
	fmt.Println("  server    Manage server configurations")
	fmt.Println("    list                        List all servers")
	fmt.Println("    add                         Add a server")
	fmt.Println("      --name <name>             Server name")
	fmt.Println("      --host <host>             Server host")
	fmt.Println("      --port <port>             Server port (default 22)")
	fmt.Println("      --user <user>             Username")
	fmt.Println("      --auth <type>             Auth type: key or password")
	fmt.Println("      --key-path <path>         SSH key path (for key auth)")
	fmt.Println("      --password <pass>         Password (for password auth)")
	fmt.Println("    delete <name>               Delete a server")
	fmt.Println()
	fmt.Println("  web       Start web UI")
	fmt.Println("            --local               Run in local mode")
	fmt.Println("            --bind <addr>         Bind address (default 0.0.0.0:8080)")
	fmt.Println()
	fmt.Println("  portal    High-performance port forwarding/tunneling")
	fmt.Println("            --server              Run in server mode")
	fmt.Println("            --client              Run in client mode")
	fmt.Println("            --listen <addr>       Server listen address (default :18888)")
	fmt.Println("            --token <token>       Auth token")
	fmt.Println("            --local <addr>        Local listen address (client)")
	fmt.Println("            --remote <host:port>  Remote target (client)")
	fmt.Println("            --server-addr <addr>  Portal server address (client)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Upload file directly")
	fmt.Println("  gmssh upload --source ./file.txt --target gateway:/data/")
	fmt.Println()
	fmt.Println("  # Upload via bastion")
	fmt.Println("  gmssh upload --source ./file.txt --target internal:/data/ --via bastion-hk,gateway")
	fmt.Println()
	fmt.Println("  # Port forward to internal database")
	fmt.Println("  gmssh proxy --local :3306 --remote-host internal-db --remote-port 3306 --via gateway")
	fmt.Println()
	fmt.Println("  # Add a server")
	fmt.Println("  gmssh server add --name gateway --host gw.example.com --user admin --auth key --key-path ~/.ssh/id_rsa")
	fmt.Println()
	fmt.Println("  # Start portal server")
	fmt.Println("  gmssh portal --server --listen :18888 --token my-token")
	fmt.Println()
	fmt.Println("  # Start portal client")
	fmt.Println("  gmssh portal --client --local :8080 --remote 192.168.1.10:80 --server-addr portal.example.com:18888")
}
