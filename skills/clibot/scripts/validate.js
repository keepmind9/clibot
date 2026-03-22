#!/usr/bin/env node
import chalk from 'chalk';
import { getConfigPath, fileExists } from '../lib/utils.js';
import { validateConfigFile } from '../lib/validator.js';

const configPath = getConfigPath();

console.log(chalk.blue('clibot Configuration Validator'));
console.log(chalk.blue('Config file:'), configPath);
console.log();

if (!fileExists(configPath)) {
  console.error(chalk.red('✗ Config file not found:'), configPath);
  console.log(chalk.yellow('Please run: clibot config'));
  process.exit(1);
}

const { errors, warnings } = validateConfigFile(configPath);

console.log(chalk.blue('=== Validation Results ===\n'));

if (errors.length > 0) {
  console.log(chalk.red('Errors:'));
  errors.forEach(err => console.log(chalk.red(`  ✗ ${err}`)));
}

if (warnings.length > 0) {
  console.log(chalk.yellow('Warnings:'));
  warnings.forEach(warn => console.log(chalk.yellow(`  ⚠ ${warn}`)));
}

console.log();
console.log(chalk.blue('Summary:'));
console.log(`  Errors: ${errors.length}`);
console.log(`  Warnings: ${warnings.length}`);
console.log();

if (errors.length === 0) {
  console.log(chalk.green.bold('✓ Configuration is valid!'));
  process.exit(0);
} else {
  console.log(chalk.red.bold(`✗ Configuration has ${errors.length} error(s)`));
  process.exit(1);
}
