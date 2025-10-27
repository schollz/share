package main

import (
	"bufio"
	"crypto/rand"
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/schollz/share/src/client"
	"github.com/schollz/share/src/relay"

	"github.com/spf13/cobra"
	"github.com/tyler-smith/go-bip39"
)

//go:embed web/dist install.sh
var staticFS embed.FS

var (
	Version  = "dev"
	logLevel string
	domain   string
)

var rootCmd = &cobra.Command{
	Use:     "share",
	Short:   "Secure E2E encrypted file transfer",
	Long:    "Zero-knowledge relay for end-to-end encrypted file transfers using ECDH + AES-GCM",
	Version: Version,
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the relay server",
	Long:  "Start the WebSocket relay server for E2E encrypted file transfers",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		logger := createLogger(logLevel)
		relay.Start(port, staticFS, logger)
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <file> [room]",
	Short: "Send a file to a room",
	Long:  "Send a file through E2E encryption to a specified room",
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
		client.SendFile(filePath, roomID, server)
	},
}

var receiveCmd = &cobra.Command{
	Use:   "receive [room]",
	Short: "Receive a file from a room",
	Long:  "Receive a file through E2E encryption from a specified room",
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
		client.ReceiveFile(roomID, server, output)
	},
}

func promptForRoom() string {
	fmt.Print("Enter room name: ")
	reader := bufio.NewReader(os.Stdin)
	roomID, _ := reader.ReadString('\n')
	return strings.TrimSpace(roomID)
}

func generateRandomRoom() string {
	// Generate 16 bytes of random entropy
	entropy := make([]byte, 16)
	_, err := rand.Read(entropy)
	if err != nil {
		// Fallback to a simple random name if entropy generation fails
		return fmt.Sprintf("room-%d", os.Getpid())
	}

	// Generate BIP39 mnemonic from entropy
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return fmt.Sprintf("room-%d", os.Getpid())
	}

	// Extract first 2 words
	words := strings.Split(mnemonic, " ")
	if len(words) >= 2 {
		return words[0] + "-" + words[1]
	}
	return words[0]
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
	rootCmd.PersistentFlags().StringVar(&domain, "domain", "https://share.schollz.com", "Domain name for the server")

	serveCmd.Flags().IntP("port", "p", 3001, "Port to listen on")
	sendCmd.Flags().StringP("server", "s", "", "Server URL (overrides --domain)")
	receiveCmd.Flags().StringP("server", "s", "", "Server URL (overrides --domain)")
	receiveCmd.Flags().StringP("output", "o", ".", "Output directory")

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
