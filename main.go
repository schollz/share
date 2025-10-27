package main

import (
	"fmt"
	"os"

	"copy-server/src/client"
	"copy-server/src/relay"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "copy-server",
	Short: "Secure E2E encrypted file transfer",
	Long:  "Copy Server - Zero-knowledge relay for end-to-end encrypted file transfers using ECDH + AES-GCM",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the relay server",
	Long:  "Start the WebSocket relay server for E2E encrypted file transfers",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		relay.Start(port)
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <file> <room>",
	Short: "Send a file to a room",
	Long:  "Send a file through E2E encryption to a specified room",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		roomID := args[1]
		server, _ := cmd.Flags().GetString("server")
		client.SendFile(filePath, roomID, server)
	},
}

var receiveCmd = &cobra.Command{
	Use:   "receive <room>",
	Short: "Receive a file from a room",
	Long:  "Receive a file through E2E encryption from a specified room",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		roomID := args[0]
		server, _ := cmd.Flags().GetString("server")
		output, _ := cmd.Flags().GetString("output")
		client.ReceiveFile(roomID, server, output)
	},
}

func init() {
	serveCmd.Flags().IntP("port", "p", 3001, "Port to listen on")
	sendCmd.Flags().StringP("server", "s", "ws://localhost:3001", "Server URL")
	receiveCmd.Flags().StringP("server", "s", "ws://localhost:3001", "Server URL")
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
