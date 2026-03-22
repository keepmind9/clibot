#!/usr/bin/env node
/**
 * clibot CLI entry point
 */

import { program } from 'commander';
import { startService, stopService, restartService, showStatus } from './scripts/service.js';
import { install } from './scripts/install.js';

program
  .name('clibot')
  .description('Automated deployment and setup for clibot')
  .version('1.0.0');

program
  .command('setup')
  .description('Interactive setup wizard')
  .action(async () => {
    await import('./scripts/setup.js');
  });

program
  .command('install')
  .description('Download and install clibot binary')
  .option('-v, --version <version>', 'Specific version to install', 'latest')
  .option('-p, --prefix <path>', 'Installation prefix')
  .action(async (options) => {
    await install({ version: options.version !== 'latest' ? options.version : 'latest', prefix: options.prefix || null });
  });

program
  .command('config')
  .description('Generate or edit configuration')
  .action(async () => {
    await import('./scripts/config.js');
  });

program
  .command('validate')
  .description('Validate configuration file')
  .action(async () => {
    await import('./scripts/validate.js');
  });

program
  .command('start')
  .description('Start clibot service')
  .action(async () => {
    await startService();
  });

program
  .command('stop')
  .description('Stop clibot service')
  .action(async () => {
    await stopService();
  });

program
  .command('restart')
  .description('Restart clibot service')
  .action(async () => {
    await restartService();
  });

program
  .command('status')
  .description('Show service status')
  .action(async () => {
    await showStatus();
  });

program
  .command('logs')
  .description('View clibot logs')
  .option('-f, --follow', 'Follow log output')
  .option('-n, --lines <number>', 'Number of lines to show', '50')
  .action(async (options) => {
    process.argv.push(...(options.follow ? ['-f'] : []));
    process.argv.push(...(options.lines ? ['-n', options.lines] : []));
    await import('./scripts/logs.js');
  });

program
  .command('help')
  .description('Show help information')
  .action(() => {
    program.help();
  });

// Parse arguments
program.parse(process.argv);

// Show help if no command provided
if (!process.argv.slice(2).length) {
  program.help();
}
