import type { BuildConfig, CompileBuildOptions } from "bun";
import tailwindPlugin from "bun-plugin-tailwind";
import solidPlugin from "./bun-plugin-solid";
import { readdirSync } from "fs";
import path from "path";

// Build for multiple platforms
const platforms: CompileBuildOptions[] = [
    { target: "bun-windows-x64", outfile: "unblink.exe" },
    { target: "bun-linux-x64", outfile: "unblink-linux" },
    { target: "bun-darwin-arm64", outfile: "unblink-macos" },
];

// Find all worker files
const workers: string[] = readdirSync("./backend/worker").filter(file => file.endsWith(".ts"));
const worker_entrypoints = workers.map(worker => path.join("./backend/worker", worker));

const base_options: Partial<BuildConfig> = {
    plugins: [
        solidPlugin,
        tailwindPlugin,
    ],
    naming: {
        entry: "[name]-[hash].[ext]",
        chunk: "chunks/[name]-[hash].[ext]",
        asset: "assets/[name]-[hash].[ext]",
    },
}

// Build standalone binary (embedded workers)
for (const platform of platforms) {
    await Bun.build({
        ...base_options,
        // Embed worker files into the binary
        entrypoints: ["./index.ts", ...worker_entrypoints],
        outdir: "./dist",
        compile: platform,
    });
}
