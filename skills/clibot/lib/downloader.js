/**
 * Binary downloader for clibot - archive format support
 */

import axios from 'axios';
import fs from 'fs';
import path from 'path';
import { execSync } from 'child_process';
import { getPlatform, getBinaryName, ensureDir } from './utils.js';

const REPO = 'keepmind9/clibot';
const BASE_URL = 'https://github.com';

/**
 * Get download URL for clibot archive
 * @returns {{ url: string, archiveName: string }}
 */
export function getDownloadUrl(version = 'latest') {
  const { os, arch } = getPlatform();
  const binaryName = getBinaryName();
  const versionNum = version === 'latest' ? '' : version.replace(/^v/, '');
  const archiveName = `${binaryName}-${versionNum}-${os}-${arch}.${os === 'windows' ? 'zip' : 'tar.gz'}`;

  const url = version === 'latest'
    ? `${BASE_URL}/${REPO}/releases/latest/download/${archiveName}`
    : `${BASE_URL}/${REPO}/releases/download/${version}/${archiveName}`;

  return { url, archiveName };
}

/**
 * Download clibot archive
 */
export async function downloadBinary(version = 'latest', onProgress) {
  const { url, archiveName } = getDownloadUrl(version);

  // For 'latest', resolve actual version from GitHub API to build correct filename
  let resolvedUrl = url;
  let resolvedName = archiveName;
  if (version === 'latest') {
    const tag = await getLatestVersion();
    const ver = tag.replace(/^v/, '');
    const { os, arch } = getPlatform();
    resolvedName = `clibot-${ver}-${os}-${arch}.${os === 'windows' ? 'zip' : 'tar.gz'}`;
    resolvedUrl = `${BASE_URL}/${REPO}/releases/latest/download/${resolvedName}`;
  }

  const tempDir = path.join(process.env.TMP || process.env.TMPDIR || '/tmp', 'clibot');
  ensureDir(tempDir);
  const archivePath = path.join(tempDir, resolvedName);

  try {
    const response = await axios({
      method: 'GET',
      url: resolvedUrl,
      responseType: 'stream',
      onDownloadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          const downloaded = (progressEvent.loaded / 1024 / 1024).toFixed(2);
          const total = (progressEvent.total / 1024 / 1024).toFixed(2);
          onProgress({ percent, downloaded, total });
        }
      }
    });

    const writer = fs.createWriteStream(archivePath);
    response.data.pipe(writer);

    await new Promise((resolve, reject) => {
      writer.on('finish', resolve);
      writer.on('error', reject);
    });

    // Extract archive
    console.log('Extracting archive...');
    const extractDir = path.join(tempDir, 'extracted');
    ensureDir(extractDir);

    if (resolvedName.endsWith('.zip')) {
      execSync(`powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${extractDir}' -Force"`, { stdio: 'pipe' });
    } else {
      execSync(`tar -xzf "${archivePath}" -C "${extractDir}"`, { stdio: 'pipe' });
    }

    // Find the binary in extracted directory
    const binaryName = getBinaryName();
    const entries = fs.readdirSync(extractDir);
    let binaryPath = null;

    for (const entry of entries) {
      // Validate entry name (prevent path traversal via malicious directory names)
      if (entry.includes('..') || entry.includes('/') || entry.includes('\\')) {
        continue;
      }
      const candidate = path.join(extractDir, entry, binaryName);
      // Validate binary is within extractDir
      const realCandidate = path.resolve(candidate);
      const realExtractDir = path.resolve(extractDir);
      if (!realCandidate.startsWith(realExtractDir + path.sep)) {
        continue;
      }
      if (fs.existsSync(candidate)) {
        binaryPath = candidate;
        break;
      }
    }

    if (!binaryPath) {
      throw new Error(`Binary not found in extracted archive`);
    }

    // Clean up archive
    fs.unlinkSync(archivePath);

    return binaryPath;
  } catch (error) {
    if (fs.existsSync(archivePath)) {
      fs.unlinkSync(archivePath);
    }
    throw new Error(`Failed to download: ${error.message}`);
  }
}

/**
 * Install binary to target directory
 */
export async function installBinary(sourceFile, targetDir) {
  const binaryName = getBinaryName();
  const targetFile = path.join(targetDir, binaryName);

  ensureDir(targetDir);

  fs.copyFileSync(sourceFile, targetFile);

  if (process.platform !== 'win32') {
    fs.chmodSync(targetFile, 0o755);
  }

  return targetFile;
}

/**
 * Get latest version from GitHub
 */
export async function getLatestVersion() {
  const response = await axios.get(`https://api.github.com/repos/${REPO}/releases/latest`);
  return response.data.tag_name;
}

/**
 * List available versions
 */
export async function listVersions() {
  const response = await axios.get(`https://api.github.com/repos/${REPO}/releases`, {
    params: { per_page: 10 }
  });
  return response.data.map(release => release.tag_name);
}