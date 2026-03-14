"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const packageJSON = require("../package.json");

const releaseRepo = process.env.TOKIAME_RELEASE_REPO || "Tokimorphling/Tokilake";
const releaseTag = process.env.TOKIAME_RELEASE_TAG || `v${packageJSON.version}`;

function getTarget() {
  const platform = process.platform;
  const arch = process.arch;

  if (platform === "darwin" && arch === "x64") {
    return { assetOS: "darwin", assetArch: "amd64", archiveExt: "tar.gz", checksumOS: "darwin", binaryName: "tokiame" };
  }
  if (platform === "darwin" && arch === "arm64") {
    return { assetOS: "darwin", assetArch: "arm64", archiveExt: "tar.gz", checksumOS: "darwin", binaryName: "tokiame" };
  }
  if (platform === "linux" && arch === "x64") {
    return { assetOS: "linux", assetArch: "amd64", archiveExt: "tar.gz", checksumOS: "linux", binaryName: "tokiame" };
  }
  if (platform === "linux" && arch === "arm64") {
    return { assetOS: "linux", assetArch: "arm64", archiveExt: "tar.gz", checksumOS: "linux", binaryName: "tokiame" };
  }
  if (platform === "win32" && arch === "x64") {
    return { assetOS: "windows", assetArch: "amd64", archiveExt: "zip", checksumOS: "windows", binaryName: "tokiame.exe" };
  }
  throw new Error(`unsupported platform: ${platform}/${arch}`);
}

function getAssetURL(fileName) {
  return `https://github.com/${releaseRepo}/releases/download/${releaseTag}/${fileName}`;
}

function download(url) {
  return new Promise((resolve, reject) => {
    const request = https.get(url, { headers: { "User-Agent": "@tokilake/tokiame" } }, (response) => {
      if (response.statusCode && response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        download(response.headers.location).then(resolve).catch(reject);
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`download failed: ${url} (${response.statusCode})`));
        return;
      }

      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => resolve(Buffer.concat(chunks)));
    });
    request.on("error", reject);
  });
}

function verifyChecksum(buffer, checksumText, assetName) {
  const digest = crypto.createHash("sha256").update(buffer).digest("hex");
  const expectedLine = checksumText
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find((line) => line.endsWith(` ${assetName}`));

  if (!expectedLine) {
    throw new Error(`checksum entry not found for ${assetName}`);
  }

  const expected = expectedLine.split(/\s+/)[0];
  if (expected !== digest) {
    throw new Error(`checksum mismatch for ${assetName}`);
  }
}

function extractArchive(archivePath, extractDir, archiveExt) {
  if (archiveExt === "tar.gz") {
    const result = spawnSync("tar", ["-xzf", archivePath, "-C", extractDir], { stdio: "inherit" });
    if (result.status !== 0) {
      throw new Error("failed to extract tar.gz archive");
    }
    return;
  }

  if (archiveExt === "zip") {
    const command = `Expand-Archive -LiteralPath '${archivePath.replace(/'/g, "''")}' -DestinationPath '${extractDir.replace(/'/g, "''")}' -Force`;
    const result = spawnSync("powershell", ["-NoProfile", "-NonInteractive", "-Command", command], { stdio: "inherit" });
    if (result.status !== 0) {
      throw new Error("failed to extract zip archive");
    }
    return;
  }

  throw new Error(`unsupported archive format: ${archiveExt}`);
}

async function main() {
  const target = getTarget();
  const assetName = `tokiame_${packageJSON.version}_${target.assetOS}_${target.assetArch}.${target.archiveExt}`;
  const checksumName = `SHA256SUMS-${target.checksumOS}.txt`;

  const archiveBuffer = await download(getAssetURL(assetName));
  const checksumBuffer = await download(getAssetURL(checksumName));

  verifyChecksum(archiveBuffer, checksumBuffer.toString("utf8"), assetName);

  const tempDir = await fsp.mkdtemp(path.join(os.tmpdir(), "tokiame-install-"));
  const archivePath = path.join(tempDir, assetName);
  const extractDir = path.join(tempDir, "extract");

  await fsp.mkdir(extractDir, { recursive: true });
  await fsp.writeFile(archivePath, archiveBuffer);
  extractArchive(archivePath, extractDir, target.archiveExt);

  const sourceBinaryPath = path.join(extractDir, target.binaryName);
  const destinationDir = path.join(__dirname, "..", "bin");
  const destinationPath = path.join(destinationDir, target.binaryName);

  if (!fs.existsSync(sourceBinaryPath)) {
    throw new Error(`extracted binary not found: ${sourceBinaryPath}`);
  }

  await fsp.mkdir(destinationDir, { recursive: true });
  await fsp.copyFile(sourceBinaryPath, destinationPath);

  if (process.platform !== "win32") {
    await fsp.chmod(destinationPath, 0o755);
  }

  const homeDir = os.homedir();
  if (homeDir) {
    const configDir = path.join(homeDir, ".tokilake");
    const exampleSource = path.join(__dirname, "..", "tokiame.json.example");
    const exampleDestination = path.join(configDir, "tokiame.json.example");
    await fsp.mkdir(configDir, { recursive: true });
    if (fs.existsSync(exampleSource) && !fs.existsSync(exampleDestination)) {
      await fsp.copyFile(exampleSource, exampleDestination);
    }
  }
}

main().catch((error) => {
  console.error(`[tokiame] ${error.message}`);
  process.exit(1);
});
