/**
 * Binary downloader for clibot
 */

import axios from 'axios';
import fs from 'fs';
import path from 'path';
import { getPlatform, getBinaryName, ensureDir } from './utils.js';

const REPO = 'keepmind9/clibot';
const BASE_URL = 'https://github.com';

/**
 * Get download URL and binary name for clibot binary
 * @returns {{ url: string, binaryName: string }}
 */
export function getDownloadUrl(version = 'latest') {
  const { os, arch } = getPlatform();
  const binaryName = getBinaryName();
  const filename = `${os}-${arch}-${binaryName}`;

  const url = version === 'latest'
    ? `${BASE_URL}/${REPO}/releases/latest/download/${filename}`
    : `${BASE_URL}/${REPO}/releases/download/${version}/${filename}`;

  return { url, binaryName };
}

/**
 * Download clibot binary
 */
export async function downloadBinary(version = 'latest', onProgress) {
  const { url, binaryName } = getDownloadUrl(version);

  const tempDir = path.join(process.env.TMP || process.env.TMPDIR || '/tmp', 'clibot');
  ensureDir(tempDir);
  const tempFile = path.join(tempDir, binaryName);

  console.log(`Downloading from: ${url}`);

  try {
    const response = await axios({
      method: 'GET',
      url,
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

    const writer = fs.createWriteStream(tempFile);
    response.data.pipe(writer);

    return new Promise((resolve, reject) => {
      writer.on('finish', () => resolve(tempFile));
      writer.on('error', reject);
    });
  } catch (error) {
    if (fs.existsSync(tempFile)) {
      fs.unlinkSync(tempFile);
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
