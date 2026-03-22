#!/usr/bin/env node
import chalk from 'chalk';
import inquirer from 'inquirer';
import { getConfigPath, writeFile, fileExists } from '../lib/utils.js';

async function generateConfig() {
  console.log(chalk.blue('Configuration Generator\n'));

  const answers = await inquirer.prompt([
    {
      type: 'input',
      name: 'sessionName',
      message: 'Session name:',
      default: 'claude'
    },
    {
      type: 'list',
      name: 'cliType',
      message: 'CLI type:',
      choices: ['claude', 'gemini', 'opencode'],
      default: 'claude'
    },
    {
      type: 'input',
      name: 'workDir',
      message: 'Work directory:',
      default: process.env.HOME + '/work'
    },
    {
      type: 'input',
      name: 'startCmd',
      message: 'Start command:',
      default: 'claude'
    },
    {
      type: 'confirm',
      name: 'telegramEnabled',
      message: 'Enable Telegram?',
      default: true
    },
    {
      type: 'password',
      name: 'telegramToken',
      message: 'Telegram Bot Token:',
      when: (answers) => answers.telegramEnabled,
      validate: (input) => /^\d+:[A-Za-z0-9_-]+$/.test(input) || 'Invalid token format'
    },
    {
      type: 'input',
      name: 'adminIds',
      message: 'Admin user IDs (comma-separated):',
      default: ''
    }
  ]);

  const config = {
    sessions: [{
      name: answers.sessionName,
      cli_type: answers.cliType,
      cli_adapter: 'acp',
      work_dir: answers.workDir,
      start_cmd: answers.startCmd,
      auto_start: true,
      created_by: 'clibot'
    }],
    telegram: {
      enabled: answers.telegramEnabled,
      token: answers.telegramEnabled ? answers.telegramToken : '',
      webhook_url: ''
    },
    discord: {
      enabled: false,
      token: '',
      guild_id: ''
    },
    feishu: {
      enabled: false,
      app_id: '',
      app_secret: ''
    },
    admins: answers.adminIds ? answers.adminIds.split(',').map(id => id.trim()) : [],
    whitelist: {
      enabled: false,
      users: []
    }
  };

  const configPath = getConfigPath();
  const yamlContent = yaml.dump(config, { indent: 2 });

  console.log(chalk.yellow('\nGenerated configuration:'));
  console.log(chalk.gray('─'.repeat(60)));
  console.log(yamlContent.replace(/token:.+/, 'token: [REDACTED]'));
  console.log(chalk.gray('─'.repeat(60)));

  const confirm = await inquirer.prompt([
    {
      type: 'confirm',
      name: 'save',
      message: 'Save this configuration?',
      default: true
    }
  ]);

  if (confirm.save) {
    writeFile(configPath, yamlContent);
    console.log(chalk.green(`\n✓ Configuration saved to: ${configPath}`));
    console.log(chalk.cyan('Next steps:'));
    console.log(chalk.cyan('  1. Run: clibot validate'));
    console.log(chalk.cyan('  2. Run: clibot start'));
  } else {
    console.log(chalk.yellow('\nConfiguration not saved.'));
  }
}

if (fileExists(getConfigPath())) {
  console.log(chalk.yellow('Existing config found at:'), getConfigPath());
  console.log(chalk.cyan('Edit it manually or run again to overwrite.\n'));
}

generateConfig().catch(error => {
  console.error(chalk.red('Error:'), error.message);
  process.exit(1);
});
