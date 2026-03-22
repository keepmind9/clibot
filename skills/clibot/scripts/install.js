#!/usr/bin/env node
/**
 * clibot binary installer
 */

import chalk from 'chalk';
import ora from 'ora';
import path from 'path';
import fs from 'fs';
import { exec } from 'child_process';
import { getPlatform, getBinaryName, getWritableExecutablePath, fileExists } from '../lib/utils.js';
import { downloadBinary, installBinary } from '../lib/downloader.js';

/**
 * Install clibot binary
 * @param {{ version?: string, prefix?: string }} options
 */
export async function install({ version = 'latest', prefix = null } = {}) {
  console.log(chalk.blue.bold('=== clibot Installer ===\n'));

  const { os, arch, platform } = getPlatform();
  console.log(chalk.green(`✓ Detected OS: ${os}`));
  console.log(chalk.green(`✓ Detected architecture: ${arch}`));

  const installDir = prefix || getWritableExecutablePath();
  console.log(chalk.green(`✓ Install directory: ${installDir}`));

  const binaryName = getBinaryName();
  const targetFile = path.join(installDir, binaryName);

  if (fileExists(targetFile)) {
    console.log(chalk.yellow(`⚠ Binary already exists at: ${targetFile}`));
    console.log(chalk.yellow('It will be overwritten.\n'));
  }

  // Download
  let spinner = ora('Downloading binary...').start();
  let tempFile;

  try {
    tempFile = await downloadBinary(version, ({ percent, downloaded, total }) => {
      spinner.text = `Downloading ${binaryName}... ${percent}% (${downloaded}/${total} MB)`;
    });
    spinner.succeed(chalk.green('Binary downloaded successfully'));
  } catch (error) {
    spinner.fail(chalk.red('Failed to download binary'));
    console.error(chalk.red(error.message));
    console.log(chalk.yellow('\nPlease check:'));
    console.log('  - Internet connection');
    console.log('  - GitHub accessibility');
    console.log(`  - Version exists: https://github.com/keepmind9/clibot/releases`);
    process.exit(1);
  }

  // Install
  spinner = ora('Installing binary...').start();
  try {
    const installedPath = await installBinary(tempFile, installDir);
    spinner.succeed(chalk.green(`✓ Binary installed to: ${installedPath}`));
    fs.unlinkSync(tempFile);
  } catch (error) {
    spinner.fail(chalk.red('Failed to install binary'));
    console.error(chalk.red(error.message));
    process.exit(1);
  }

  // Verify
  spinner = ora('Verifying installation...').start();
  return new Promise((resolve, reject) => {
    exec(`"${targetFile}" --version`, (error, stdout) => {
      if (error) {
        spinner.fail(chalk.red('Installation verification failed'));
        console.log(chalk.yellow('\nBinary may not be compatible with your system'));
        reject(error);
        return;
      }

      spinner.succeed(chalk.green('✓ Installation verified successfully'));
      console.log(chalk.cyan(`\n${stdout.trim()}`));

      const pathEnv = process.env.PATH || process.env.Path || '';
      if (!pathEnv.includes(installDir)) {
        console.log(chalk.yellow(`\n⚠ Warning: ${installDir} is not in PATH`));
        console.log(chalk.yellow('Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):\n'));
        if (platform === 'win32') {
          console.log(chalk.cyan(`  setx PATH "%PATH%;${installDir}"`));
        } else {
          console.log(chalk.cyan(`  export PATH="\\$PATH:${installDir}"`));
        }
        console.log(chalk.yellow('\nThen restart your shell or run:'));
        if (platform === 'win32') {
          console.log(chalk.cyan('  refreshenv'));
        } else {
          console.log(chalk.cyan('  source ~/.bashrc  # or source ~/.zshrc'));
        }
      }

      console.log(chalk.green('\n=== Installation Complete ==='));
      console.log(chalk.green('Next steps:'));
      console.log(chalk.cyan('  1. Run: clibot setup'));
      console.log(chalk.cyan('  2. Or manually: clibot config'));

      resolve();
    });
  });
}

// CLI entry point
if (process.argv[1] && process.argv[1].endsWith('install.js')) {
  const args = process.argv.slice(2);
  let version = 'latest';
  let prefix = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--version' && args[i + 1]) {
      version = args[++i];
    } else if (args[i] === '--prefix' && args[i + 1]) {
      prefix = args[++i];
    } else if (args[i] === '--help' || args[i] === '-h') {
      console.log('Usage: clibot install [--version <version>] [--prefix <path>]\n');
      console.log('Options:');
      console.log('  --version <version>  Specific version to install (default: latest)');
      console.log('  --prefix <path>     Installation prefix (default: auto-detected)');
      console.log('  --help, -h          Show this help message\n');
      console.log('Examples:');
      console.log('  clibot install                          # Install latest version');
      console.log('  clibot install --version v0.1.0        # Install specific version');
      console.log('  clibot install --prefix ~/bin          # Install to custom location\n');
      process.exit(0);
    }
  }

  install({ version, prefix }).catch(error => {
    console.error(chalk.red('Error:'), error.message);
    process.exit(1);
  });
}
