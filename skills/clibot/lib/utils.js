/**
 * Utility functions for clibot
 */

import os from 'os';
import path from 'path';
import fs from 'fs';

/**
 * Get platform information
 */
export function getPlatform() {
  const platform = os.platform(); // 'win32', 'darwin', 'linux'
  const arch = os.arch(); // 'x64', 'arm64', etc.

  let osName;
  switch (platform) {
    case 'win32':
      osName = 'windows';
      break;
    case 'darwin':
      osName = 'darwin';
      break;
    case 'linux':
      osName = 'linux';
      break;
    default:
      throw new Error(`Unsupported platform: ${platform}`);
  }

  let archName;
  switch (arch) {
    case 'x64':
    case 'amd64':
      archName = 'amd64';
      break;
    case 'arm64':
    case 'aarch64':
      archName = 'arm64';
      break;
    case 'ia32':
    case 'x86':
      archName = '386';
      break;
    default:
      throw new Error(`Unsupported architecture: ${arch}`);
  }

  return { os: osName, arch: archName, platform, originalArch: arch };
}

/**
 * Get clibot config directory
 */
export function getConfigDir() {
  const homeDir = os.homedir();
  return path.join(homeDir, '.clibot');
}

/**
 * Get clibot config file path
 */
export function getConfigPath() {
  return path.join(getConfigDir(), 'config.yaml');
}

/**
 * Get clibot PID file path
 */
export function getPidFilePath() {
  return path.join(getConfigDir(), 'clibot.pid');
}

/**
 * Get clibot log directory
 */
export function getLogDir() {
  return path.join(getConfigDir(), 'logs');
}

/**
 * Get clibot log file path
 */
export function getLogFilePath() {
  return path.join(getLogDir(), 'clibot.log');
}

/**
 * Ensure directory exists, create if not
 */
export function ensureDir(dirPath) {
  if (!fs.existsSync(dirPath)) {
    fs.mkdirSync(dirPath, { recursive: true });
  }
}

/**
 * Check if file exists
 */
export function fileExists(filePath) {
  try {
    return fs.existsSync(filePath);
  } catch {
    return false;
  }
}

/**
 * Read file safely
 */
export function readFile(filePath) {
  try {
    return fs.readFileSync(filePath, 'utf-8');
  } catch (error) {
    throw new Error(`Failed to read file ${filePath}: ${error.message}`);
  }
}

/**
 * Write file safely
 */
export function writeFile(filePath, content, mode = 0o600) {
  try {
    ensureDir(path.dirname(filePath));
    fs.writeFileSync(filePath, content, { mode });
  } catch (error) {
    throw new Error(`Failed to write file ${filePath}: ${error.message}`);
  }
}

/**
 * Get executable paths in order of preference
 */
export function getExecutablePaths() {
  const paths = [];

  // User local bin (no sudo required)
  if (process.env.LOCALAPPDATA) {
    // Windows
    paths.push(path.join(process.env.LOCALAPPDATA, 'clibot'));
  }
  paths.push(path.join(os.homedir(), '.local', 'bin'));
  paths.push(path.join(os.homedir(), 'bin'));

  // System bin (may require sudo)
  if (process.env.ProgramFiles) {
    // Windows
    paths.push(path.join(process.env.ProgramFiles, 'clibot'));
  }
  paths.push('/usr/local/bin');
  paths.push('/usr/bin');

  return paths;
}

/**
 * Find writable executable path
 */
export function getWritableExecutablePath() {
  const paths = getExecutablePaths();

  for (const p of paths) {
    try {
      ensureDir(p);
      // Test write permissions
      const testFile = path.join(p, '.write-test');
      fs.writeFileSync(testFile, 'test');
      fs.unlinkSync(testFile);
      return p;
    } catch {
      continue;
    }
  }

  // Fallback to temp directory
  return os.tmpdir();
}

/**
 * Get binary name for platform
 */
export function getBinaryName() {
  const { os } = getPlatform();
  return os === 'windows' ? 'clibot.exe' : 'clibot';
}

/**
 * Check if command exists
 */
export async function commandExists(cmd) {
  try {
    const { exec } = await import('child_process');
    return new Promise((resolve) => {
      exec(`which ${cmd}`, { shell: true }, (error) => {
        resolve(!error);
      });
    });
  } catch {
    return false;
  }
}

/**
 * Execute shell command
 */
export async function execCommand(cmd, options = {}) {
  const { exec } = await import('child_process');
  return new Promise((resolve, reject) => {
    exec(cmd, { shell: true, ...options }, (error, stdout, stderr) => {
      if (error) {
        reject(new Error(`Command failed: ${error.message}\n${stderr}`));
      } else {
        resolve(stdout.trim());
      }
    });
  });
}

/**
 * Read process PID from file
 */
export function readPidFile() {
  const pidFile = getPidFilePath();
  if (!fileExists(pidFile)) {
    return null;
  }

  const pid = parseInt(readFile(pidFile).trim(), 10);
  return isNaN(pid) ? null : pid;
}

/**
 * Check if process is running
 */
export async function isProcessRunning(pid) {
  if (!pid) return false;

  try {
    process.kill(pid, 0); // Signal 0 checks if process exists
    return true;
  } catch {
    return false;
  }
}

/**
 * Sleep for specified milliseconds
 */
export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Format bytes to human readable
 */
export function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

/**
 * Format duration to human readable
 */
export function formatDuration(seconds) {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);

  if (hours > 0) {
    return `${hours}h ${minutes}m ${secs}s`;
  } else if (minutes > 0) {
    return `${minutes}m ${secs}s`;
  } else {
    return `${secs}s`;
  }
}
