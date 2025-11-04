package main

import (
	"bufio"
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/schollz/e2ecp/src/client"
	"github.com/schollz/e2ecp/src/relay"

	"github.com/spf13/cobra"
)

//go:embed all:web/dist install.sh
var staticFS embed.FS

var (
	Version  = "dev"
	logLevel string
	domain   string
)

var rootCmd = &cobra.Command{
	Use:     "e2ecp",
	Short:   "Secure E2E encrypted file transfer",
	Long:    "Zero-knowledge relay for end-to-end encrypted file transfers using ECDH + AES-GCM",
	Version: Version,
	Example: `  # Send a file (generates random room name)
  e2ecp send myfile.txt

  # Send a file to a specific room
  e2ecp send myfile.txt cool-room

  # Send a folder (automatically zipped)
  e2ecp send ./my-folder

  # Receive a file from a room
  e2ecp receive cool-room

  # Receive to a specific directory
  e2ecp receive cool-room -o ~/Downloads

  # Start your own relay server
  e2ecp serve -p 3001`,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the relay server",
	Long:  "Start the WebSocket relay server for E2E encrypted file transfers",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		maxRooms, _ := cmd.Flags().GetInt("max-rooms")
		maxRoomsPerIP, _ := cmd.Flags().GetInt("max-rooms-per-ip")
		logger := createLogger(logLevel)
		relay.Start(port, maxRooms, maxRoomsPerIP, staticFS, logger)
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <file-or-folder> [room]",
	Short: "Send a file or folder to a room",
	Long:  "Send a file or folder through E2E encryption to a specified room. Folders are automatically zipped for transfer.",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		var roomID string
		if len(args) >= 2 {
			roomID = args[1]
		} else {
			roomID = generateRandomRoom()
		}
		server, _ := cmd.Flags().GetString("server")
		if server == "" {
			server = getWebSocketURL(domain)
		}
		logger := createLogger(logLevel)
		client.SendFile(filePath, roomID, server, logger)
	},
}

var receiveCmd = &cobra.Command{
	Use:   "receive [room]",
	Short: "Receive a file or folder from a room",
	Long:  "Receive a file or folder through E2E encryption from a specified room. Folders are automatically extracted.",
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		var roomID string
		if len(args) >= 1 {
			roomID = args[0]
		} else {
			roomID = promptForRoom()
		}
		server, _ := cmd.Flags().GetString("server")
		if server == "" {
			server = getWebSocketURL(domain)
		}
		output, _ := cmd.Flags().GetString("output")
		force, _ := cmd.Flags().GetBool("force")
		logger := createLogger(logLevel)
		client.ReceiveFile(roomID, server, output, force, logger)
	},
}

func promptForRoom() string {
	fmt.Print("Enter room name: ")
	reader := bufio.NewReader(os.Stdin)
	roomID, _ := reader.ReadString('\n')
	return strings.TrimSpace(roomID)
}

func generateRandomRoom() string {
	// Generate a random 3-word icon-based room name
	return relay.GenerateRandomIconMnemonic(3)
}

func getWebSocketURL(domain string) string {
	if domain == "" || domain == "https://" {
		return "wss://"
	} else if domain == "http://" {
		return "ws://"
	}
	// }
	// Parse the URL
	u, err := url.Parse(domain)
	if err != nil || u.Scheme == "" {
		// If parsing fails or no scheme, assume https
		u, _ = url.Parse("https://" + domain)
	}

	// Convert http/https schemes to ws/wss
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		// For any other scheme or empty, default to wss
		u.Scheme = "wss"
	}

	return u.String()
}

func createLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: logLevel}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&domain, "domain", "https://e2ecp.com", "Domain name for the server")

	serveCmd.Flags().IntP("port", "p", 3001, "Port to listen on")
	serveCmd.Flags().Int("max-rooms", 10, "Maximum number of concurrent rooms allowed on the server")
	serveCmd.Flags().Int("max-rooms-per-ip", 2, "Maximum number of rooms per IP address")
	sendCmd.Flags().StringP("server", "s", "", "Server URL (overrides --domain)")
	receiveCmd.Flags().StringP("server", "s", "", "Server URL (overrides --domain)")
	receiveCmd.Flags().StringP("output", "o", ".", "Output directory")
	receiveCmd.Flags().BoolP("force", "f", false, "Force overwrite existing files without prompting")

	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(receiveCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
