#!/usr/bin/env node
import chalk from 'chalk';
import { getLogFilePath, fileExists } from '../lib/utils.js';
import { exec } from 'child_process';

const args = process.argv.slice(2);
const follow = args.includes('-f') || args.includes('--follow');
const linesIndex = args.indexOf('-n');
const lines = linesIndex !== -1 ? args[linesIndex + 1] || '50' : '50';

const logFile = getLogFilePath();

if (!fileExists(logFile)) {
  console.log(chalk.yellow('Log file not found:'), logFile);
  console.log(chalk.yellow('Service may not have been started yet.'));
  process.exit(1);
}

console.log(chalk.blue(`clibot logs: ${logFile}\n`));

const cmd = follow ? `tail -f "${logFile}"` : `tail -n ${lines} "${logFile}"`;
const proc = exec(cmd, { shell: true });

proc.stdout.pipe(process.stdout);
proc.stderr.pipe(process.stderr);

// Handle exit on Ctrl+C
process.on('SIGINT', () => {
  console.log(chalk.yellow('\n[Stopped]'));
  proc.kill();
  process.exit(0);
});
