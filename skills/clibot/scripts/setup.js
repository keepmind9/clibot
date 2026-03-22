#!/usr/bin/env node
import chalk from 'chalk';
import ora from 'ora';
import inquirer from 'inquirer';
import yaml from 'js-yaml';
import { getConfigPath, writeFile, fileExists, ensureDir, commandExists, getWritableExecutablePath } from '../lib/utils.js';
import { downloadBinary, installBinary } from '../lib/downloader.js';
import { startService } from './service.js';
import path from 'path';
import fs from 'fs';

async function ensureClibotInstalled() {
  const installed = await commandExists('clibot');
  if (installed) {
    console.log(chalk.green('✓ clibot binary found'));
    return;
  }

  console.log(chalk.yellow('⚠ clibot binary not found, installing now...\n'));

  const spinner = ora('Downloading clibot...');
  spinner.start();

  let tempFile;
  try {
    tempFile = await downloadBinary('latest');
    spinner.text = 'Installing clibot...';
    const installDir = getWritableExecutablePath();
    await installBinary(tempFile, installDir);
    spinner.succeed(chalk.green('✓ clibot installed to ~/.local/bin'));
  } catch (err) {
    spinner.fail(chalk.red('✗ Installation failed: ' + err.message));
    console.log(chalk.yellow('\nTip: You can install manually with:'));
    console.log(chalk.cyan('  curl -LO https://github.com/keepmind9/clibot/releases/latest/download/clibot-linux-amd64'));
    console.log(chalk.cyan('  chmod +x clibot-linux-amd64 && mv clibot-linux-amd64 ~/.local/bin/clibot\n'));
    process.exit(1);
  }
}

async function welcome() {
  console.log(chalk.blue.bold('\n=== Clibot Setup Wizard ===\n'));

  await ensureClibotInstalled();

  console.log();
  console.log('This wizard will guide you through configuring clibot.\n');

  const configPath = getConfigPath();

  if (fileExists(configPath)) {
    const { overwrite } = await inquirer.prompt([
      {
        type: 'confirm',
        name: 'overwrite',
        message: 'Existing config found. Overwrite?',
        default: false
      }
    ]);

    if (!overwrite) {
      console.log(chalk.yellow('Setup cancelled. Existing config preserved.\n'));
      process.exit(0);
    }
  }

  ensureDir(path.dirname(configPath));
}

async function selectPlatforms() {
  console.log(chalk.blue('Step 1: Select IM Platforms\n'));
  
  const platforms = await inquirer.prompt([
    {
      type: 'checkbox',
      name: 'enabled',
      message: 'Which platforms do you want to enable?',
      choices: [
        { name: 'Telegram', value: 'telegram', checked: true },
        { name: 'Discord', value: 'discord', checked: false },
        { name: 'Feishu/Lark', value: 'feishu', checked: false }
      ],
      validate: (input) => input.length > 0 || 'At least one platform must be enabled'
    }
  ]);
  
  return platforms.enabled;
}

async function collectBotTokens(platforms) {
  console.log(chalk.blue('\nStep 2: Configure Bot Tokens\n'));
  
  const tokens = { telegram: '', discord: '', feishu: { app_id: '', app_secret: '' } };
  
  if (platforms.includes('telegram')) {
    console.log(chalk.cyan('Telegram Bot Setup'));
    console.log(chalk.gray('1. Open https://t.me/BotFather'));
    console.log(chalk.gray('2. Send /newbot'));
    console.log(chalk.gray('3. Copy the token (format: 123456:ABC-DEF...)'));
    console.log();
    
    while (true) {
      const { token } = await inquirer.prompt([
        {
          type: 'password',
          name: 'token',
          message: 'Enter your Telegram Bot Token:',
          validate: (input) => {
            if (/^\d+:[A-Za-z0-9_-]+$/.test(input)) return true;
            return 'Invalid format. Expected: 123456:ABC-DEF...';
          }
        }
      ]);
      
      tokens.telegram = token;
      console.log(chalk.green('✓ Token validated\n'));
      break;
    }
  }
  
  if (platforms.includes('discord')) {
    console.log(chalk.cyan('Discord Bot Setup'));
    console.log(chalk.gray('1. Go to https://discord.com/developers/applications'));
    console.log(chalk.gray('2. Create bot and get token'));
    console.log(chalk.gray('3. Enable Message Content Intent'));
    console.log();
    
    const { token } = await inquirer.prompt([
      {
        type: 'password',
        name: 'token',
        message: 'Enter your Discord Bot Token:',
        validate: (input) => {
          if (input.length >= 59) return true;
          return 'Token too short. Discord tokens are 59+ characters.';
        }
      }
    ]);
    
    tokens.discord = token;
    console.log(chalk.green('✓ Token validated\n'));
  }
  
  if (platforms.includes('feishu')) {
    console.log(chalk.cyan('Feishu Bot Setup'));
    console.log(chalk.gray('1. Go to https://open.feishu.cn/app'));
    console.log(chalk.gray('2. Create app and get credentials'));
    console.log();
    
    const feishu = await inquirer.prompt([
      {
        type: 'input',
        name: 'app_id',
        message: 'Enter your Feishu App ID:',
        validate: (input) => input.length > 0 || 'App ID is required'
      },
      {
        type: 'password',
        name: 'app_secret',
        message: 'Enter your Feishu App Secret:',
        validate: (input) => input.length > 0 || 'App Secret is required'
      }
    ]);
    
    tokens.feishu = feishu;
    console.log(chalk.green('✓ Credentials saved\n'));
  }
  
  return tokens;
}

async function configureSession() {
  console.log(chalk.blue('Step 3: Configure Claude Session\n'));
  
  return await inquirer.prompt([
    {
      type: 'input',
      name: 'name',
      message: 'Session name:',
      default: 'claude'
    },
    {
      type: 'list',
      name: 'cli_type',
      message: 'CLI type:',
      choices: ['claude', 'gemini', 'opencode'],
      default: 'claude'
    },
    {
      type: 'input',
      name: 'work_dir',
      message: 'Work directory:',
      default: path.join(process.env.HOME, 'work')
    },
    {
      type: 'input',
      name: 'start_cmd',
      message: 'Start command:',
      default: 'claude'
    }
  ]);
}

async function configurePermissions() {
  console.log(chalk.blue('\nStep 4: Configure Permissions\n'));
  
  console.log('Admin users can perform sensitive operations.');
  console.log('Send /echo to your bot after starting to get your user ID.\n');
  
  const { admin_ids } = await inquirer.prompt([
    {
      type: 'input',
      name: 'admin_ids',
      message: 'Enter admin user IDs (comma-separated, or leave empty):'
    }
  ]);
  
  const admins = admin_ids ? admin_ids.split(',').map(id => id.trim()) : [];
  
  const { whitelist_enabled } = await inquirer.prompt([
    {
      type: 'confirm',
      name: 'whitelist_enabled',
      message: 'Enable whitelist mode?',
      default: false
    }
  ]);
  
  let whitelist_users = [];
  if (whitelist_enabled) {
    const { users } = await inquirer.prompt([
      {
        type: 'input',
        name: 'users',
        message: 'Enter whitelisted user IDs (comma-separated):'
      }
    ]);
    whitelist_users = users ? users.split(',').map(id => id.trim()) : [];
  }
  
  return { admins, whitelist_enabled, whitelist_users };
}

async function generateConfig(platforms, tokens, session, permissions) {
  console.log(chalk.blue('\nStep 5: Generate Configuration\n'));
  
  const config = {
    sessions: [{
      name: session.name,
      cli_type: session.cli_type,
      cli_adapter: 'acp',
      work_dir: session.work_dir,
      start_cmd: session.start_cmd,
      auto_start: true,
      created_by: 'clibot'
    }],
    telegram: {
      enabled: platforms.includes('telegram'),
      token: tokens.telegram || '',
      webhook_url: ''
    },
    discord: {
      enabled: platforms.includes('discord'),
      token: tokens.discord || '',
      guild_id: ''
    },
    feishu: {
      enabled: platforms.includes('feishu'),
      app_id: tokens.feishu.app_id || '',
      app_secret: tokens.feishu.app_secret || ''
    },
    admins: permissions.admins,
    whitelist: {
      enabled: permissions.whitelist_enabled,
      users: permissions.whitelist_users
    }
  };
  
  const configPath = getConfigPath();
  const yamlContent = yaml.dump(config, { indent: 2 });
  
  writeFile(configPath, yamlContent);
  
  console.log(chalk.green('✓ Configuration saved to:'), configPath);
  return configPath;
}

async function showSummary(config) {
  console.log(chalk.blue('\n=== Configuration Summary ===\n'));
  
  console.log(chalk.cyan('Platforms:'));
  if (config.telegram.enabled) console.log('  ✓ Telegram');
  if (config.discord.enabled) console.log('  ✓ Discord');
  if (config.feishu.enabled) console.log('  ✓ Feishu');
  
  console.log(chalk.cyan('\nSession:'));
  console.log(`  Name: ${config.sessions[0].name}`);
  console.log(`  Type: ${config.sessions[0].cli_type}`);
  console.log(`  Adapter: ${config.sessions[0].cli_adapter}`);
  console.log(`  Work dir: ${config.sessions[0].work_dir}`);
  console.log(`  Command: ${config.sessions[0].start_cmd}`);
  
  console.log(chalk.cyan('\nPermissions:'));
  console.log(`  Admins: ${config.admins.length} configured`);
  console.log(`  Whitelist: ${config.whitelist.enabled ? 'enabled' : 'disabled'}`);
  
  console.log();
}

async function main() {
  try {
    await welcome();
    
    const platforms = await selectPlatforms();
    const tokens = await collectBotTokens(platforms);
    const session = await configureSession();
    const permissions = await configurePermissions();
    
    const yamlContent = await generateConfig(platforms, tokens, session, permissions);
    const config = yaml.load(yamlContent);
    
    await showSummary(config);
    
    console.log(chalk.green.bold('✓ Setup complete!\n'));

    const { startNow } = await inquirer.prompt([
      {
        type: 'confirm',
        name: 'startNow',
        message: 'Start clibot service now?',
        default: true
      }
    ]);

    if (startNow) {
      console.log();
      await startService();
    } else {
      console.log(chalk.cyan('You can start later with:'));
      console.log(chalk.cyan('  clibot start\n'));
    }
    
  } catch (error) {
    console.error(chalk.red('Error:'), error.message);
    process.exit(1);
  }
}

main();
