#!/usr/bin/env node

const { spawn } = require("node:child_process");
const { existsSync } = require("node:fs");
const path = require("node:path");

const binaryName = process.platform === "win32" ? "tokiame.exe" : "tokiame";
const binaryPath = path.join(__dirname, binaryName);

if (!existsSync(binaryPath)) {
  console.error("tokiame binary is missing. Reinstall @tokilake/tokiame.");
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
});

child.on("error", (error) => {
  console.error(error.message);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
