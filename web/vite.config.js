import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { compression } from "vite-plugin-compression2";

export default defineConfig({
    plugins: [
        react(),
        compression({
            algorithm: "gzip",
            exclude: [/\.(br)$/, /\.(gz)$/],
            threshold: 1024, // Only compress files larger than 1KB
            deleteOriginalAssets: false
        })
    ],
    preview: {
        port: 5173
    },
    server: {
        port: 5173
    },
    build: {
        // Generate hashed filenames for better caching
        rollupOptions: {
            output: {
                entryFileNames: "assets/[name]-[hash].js",
                chunkFileNames: "assets/[name]-[hash].js",
                assetFileNames: "assets/[name]-[hash].[ext]"
            }
        }
    }
});
