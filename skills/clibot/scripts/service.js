/**
 * clibot service manager
 */

import chalk from 'chalk';
import ora from 'ora';
import fs from 'fs';
import path from 'path';
import { exec } from 'child_process';
import { readPidFile, isProcessRunning, getLogFilePath, getLogDir, getConfigPath, fileExists, sleep } from '../lib/utils.js';

const CLIBOT_BIN = 'clibot';

async function findClibotBinary() {
  return new Promise((resolve) => {
    exec(`which ${CLIBOT_BIN}`, { shell: true }, (error, stdout) => {
      resolve(error ? null : stdout.trim());
    });
  });
}

async function startService(configPath) {
  console.log(chalk.blue('Starting clibot service...'));

  const clibotBin = await findClibotBinary();
  if (!clibotBin) {
    console.error(chalk.red('✗ clibot binary not found'));
    console.log(chalk.yellow('Please install first: clibot install'));
    process.exit(1);
  }

  const config = configPath || getConfigPath();
  if (!fileExists(config)) {
    console.error(chalk.red('✗ Config file not found:'), config);
    console.log(chalk.yellow('Please run: clibot config'));
    process.exit(1);
  }

  const pid = readPidFile();
  if (pid && await isProcessRunning(pid)) {
    console.log(chalk.yellow(`⚠ Service is already running (PID: ${pid})`));
    return;
  }

  const logDir = getLogDir();
  fs.mkdirSync(logDir, { recursive: true });

  const logFile = getLogFilePath();
  const spinner = ora('Starting service...').start();

  try {
    const { spawn } = await import('child_process');

    const out = fs.openSync(logFile, 'a');
    const err = fs.openSync(logFile, 'a');

    const child = spawn(clibotBin, ['--config', config], {
      detached: true,
      stdio: ['ignore', out, err],
      shell: process.platform === 'win32'
    });

    const pidFile = path.join(getLogDir(), 'clibot.pid');
    fs.writeFileSync(pidFile, child.pid.toString());

    child.unref();

    await sleep(2000);

    const newPid = readPidFile();
    if (newPid && await isProcessRunning(newPid)) {
      spinner.succeed(chalk.green(`✓ Service started successfully (PID: ${newPid})`));
      console.log(chalk.cyan(`Log file: ${logFile}`));
      console.log(chalk.cyan('Follow logs: clibot logs'));
    } else {
      spinner.fail(chalk.red('✗ Failed to start service'));
      console.log(chalk.yellow('Check logs:'), chalk.cyan(logFile));
      process.exit(1);
    }
  } catch (error) {
    spinner.fail(chalk.red('✗ Failed to start service'));
    console.error(chalk.red(error.message));
    process.exit(1);
  }
}

async function stopService() {
  console.log(chalk.blue('Stopping clibot service...'));

  const pid = readPidFile();
  if (!pid) {
    console.log(chalk.yellow('⚠ Service is not running'));
    return;
  }

  if (!await isProcessRunning(pid)) {
    console.log(chalk.yellow('⚠ Service is not running (cleaning up PID file)'));
    fs.unlinkSync(path.join(getLogDir(), 'clibot.pid'));
    return;
  }

  const spinner = ora('Stopping service...').start();

  try {
    process.kill(pid, 'SIGTERM');

    let count = 0;
    while (await isProcessRunning(pid) && count < 10) {
      await sleep(1000);
      count++;
    }

    if (await isProcessRunning(pid)) {
      console.log(chalk.yellow('⚠ Graceful shutdown timeout, forcing...'));
      process.kill(pid, 'SIGKILL');
      await sleep(1000);
    }

    const pidFile = path.join(getLogDir(), 'clibot.pid');
    fs.unlinkSync(pidFile);

    if (await isProcessRunning(pid)) {
      spinner.fail(chalk.red('✗ Failed to stop service'));
      process.exit(1);
    } else {
      spinner.succeed(chalk.green('✓ Service stopped successfully'));
    }
  } catch (error) {
    spinner.fail(chalk.red('✗ Failed to stop service'));
    console.error(chalk.red(error.message));
    process.exit(1);
  }
}

async function showStatus() {
  console.log(chalk.blue.bold('clibot Service Status'));
  console.log(chalk.blue('=====================\n'));

  const clibotBin = await findClibotBinary();
  if (!clibotBin) {
    console.error(chalk.red('✗ clibot binary not found'));
    console.log(chalk.yellow('Please install first: clibot install\n'));
    return;
  }

  const pid = readPidFile();

  if (pid && await isProcessRunning(pid)) {
    console.log(chalk.green('✓ Service is running\n'));
    console.log(chalk.cyan(`PID: ${pid}`));

    if (process.platform !== 'win32') {
      try {
        const ps = await new Promise((resolve, reject) => {
          exec(`ps -p ${pid} -o pid,etime,%mem,%cpu,cmd`, (error, stdout) => {
            if (error) reject(error);
            else resolve(stdout);
          });
        });
        console.log(chalk.cyan(ps));
      } catch {
        // Ignore
      }
    }

    const logFile = getLogFilePath();
    if (fs.existsSync(logFile)) {
      console.log(chalk.cyan('\nRecent log entries:'));
      console.log(chalk.cyan('───────────────────'));

      try {
        const logs = await new Promise((resolve, reject) => {
          exec(`tail -n 10 "${logFile}"`, (error, stdout) => {
            if (error) reject(error);
            else resolve(stdout);
          });
        });
        console.log(chalk.gray(logs));
      } catch {
        console.log(chalk.yellow('Unable to read log file'));
      }
    }
  } else {
    console.log(chalk.yellow('⚠ Service is not running'));
  }

  console.log();
  console.log(chalk.cyan(`Config: ${getConfigPath()}`));
  console.log(chalk.cyan(`Log: ${getLogFilePath()}`));
  console.log(chalk.cyan(`PID file: ${path.join(getLogDir(), 'clibot.pid')}`));
}

async function restartService() {
  console.log(chalk.blue('Restarting clibot service...\n'));
  await stopService();
  console.log();
  await startService();
}

export { startService, stopService, restartService, showStatus };

async function main() {
  const command = process.argv[2] || 'help';

  switch (command) {
    case 'start':
      await startService();
      break;
    case 'stop':
      await stopService();
      break;
    case 'status':
      await showStatus();
      break;
    case 'restart':
      await restartService();
      break;
    default:
      console.log('Usage: clibot service {start|stop|status|restart}\n');
      console.log('Commands:');
      console.log('  start   - Start clibot service');
      console.log('  stop    - Stop clibot service');
      console.log('  status  - Show service status');
      console.log('  restart - Restart clibot service');
      process.exit(1);
  }
}

main().catch(error => {
  console.error(chalk.red('Error:'), error.message);
  process.exit(1);
});
