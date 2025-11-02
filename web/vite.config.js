import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { compression } from "vite-plugin-compression2";

// Plugin to inject preload hints for fonts dynamically
function preloadFontsPlugin() {
    return {
        name: 'preload-fonts',
        async transformIndexHtml(html, ctx) {
            // Only in build mode
            if (!ctx.bundle) return html;
            
            // Find font files in the bundle
            const fontFiles = Object.keys(ctx.bundle).filter(
                file => file.match(/fa-solid-900.*\.woff2$/)
            );
            
            if (fontFiles.length === 0) {
                console.warn('Warning: fa-solid-900 font not found in bundle');
                return html;
            }
            
            // Create preload links for each font
            const preloadLinks = fontFiles.map(font => 
                `    <link rel="preload" href="/${font}" as="font" type="font/woff2" crossorigin>`
            ).join('\n');
            
            const preloadSection = `
    <!-- Performance: Preload critical fonts to reduce request chain -->
${preloadLinks}`;
            
            // Inject before closing </head>
            return html.replace('</head>', `${preloadSection}\n</head>`);
        }
    };
}

export default defineConfig({
    plugins: [
        react(),
        preloadFontsPlugin(),
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
                assetFileNames: "assets/[name]-[hash].[ext]",
                // Ensure fonts are treated with proper preload
                manualChunks: undefined
            }
        },
        // Increase chunk size warning limit for font awesome
        chunkSizeWarningLimit: 600
    }
});
