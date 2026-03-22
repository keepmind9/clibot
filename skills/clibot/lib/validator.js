/**
 * Configuration validator for clibot
 */

import yaml from 'js-yaml';
import { readFile, fileExists } from './utils.js';

/**
 * Validate Telegram bot token format
 */
export function validateTelegramToken(token) {
  // Telegram tokens: 123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11
  const pattern = /^\d+:[A-Za-z0-9_-]+$/;
  return pattern.test(token);
}

/**
 * Validate Discord bot token format
 */
export function validateDiscordToken(token) {
  // Discord tokens: Base64-like, 59+ characters
  const pattern = /^[A-Za-z0-9_-]{59,}$/;
  return pattern.test(token);
}

/**
 * Validate CLI type
 */
export function validateCliType(type) {
  const validTypes = ['claude', 'gemini', 'opencode'];
  return validTypes.includes(type);
}

/**
 * Validate CLI adapter
 */
export function validateCliAdapter(adapter) {
  const validAdapters = ['acp', 'tmux', 'tmux-client'];
  return validAdapters.includes(adapter);
}

/**
 * Parse and validate YAML config
 */
export function parseConfig(configPath) {
  if (!fileExists(configPath)) {
    throw new Error(`Config file not found: ${configPath}`);
  }

  try {
    const content = readFile(configPath);
    return yaml.load(content);
  } catch (error) {
    throw new Error(`Failed to parse YAML: ${error.message}`);
  }
}

/**
 * Validate configuration object
 */
export function validateConfig(config) {
  const errors = [];
  const warnings = [];

  // Check sessions
  if (!config.sessions || !Array.isArray(config.sessions) || config.sessions.length === 0) {
    errors.push('sessions field is missing or empty');
  } else {
    config.sessions.forEach((session, index) => {
      if (!session.name) {
        errors.push(`sessions[${index}].name is required`);
      }
      if (!session.cli_type) {
        errors.push(`sessions[${index}].cli_type is required`);
      } else if (!validateCliType(session.cli_type)) {
        errors.push(`sessions[${index}].cli_type must be 'claude' or 'codex'`);
      }
      if (!session.cli_adapter) {
        errors.push(`sessions[${index}].cli_adapter is required`);
      } else if (!validateCliAdapter(session.cli_adapter)) {
        warnings.push(`sessions[${index}].cli_adapter '${session.cli_adapter}' is not recognized`);
      }
      if (!session.work_dir) {
        errors.push(`sessions[${index}].work_dir is required`);
      }
      if (!session.start_cmd) {
        errors.push(`sessions[${index}].start_cmd is required`);
      }
    });
  }

  // Check at least one bot is enabled
  const telegramEnabled = config.telegram?.enabled === true;
  const discordEnabled = config.discord?.enabled === true;
  const feishuEnabled = config.feishu?.enabled === true;

  if (!telegramEnabled && !discordEnabled && !feishuEnabled) {
    errors.push('At least one bot platform must be enabled');
  }

  // Validate Telegram
  if (telegramEnabled) {
    if (!config.telegram?.token) {
      errors.push('telegram.token is required when telegram is enabled');
    } else if (config.telegram.token === 'YOUR_TELEGRAM_BOT_TOKEN') {
      errors.push('telegram.token contains placeholder value');
    } else if (!validateTelegramToken(config.telegram.token)) {
      warnings.push('telegram.token format looks suspicious');
    }
  }

  // Validate Discord
  if (discordEnabled) {
    if (!config.discord?.token) {
      errors.push('discord.token is required when discord is enabled');
    } else if (config.discord.token === 'YOUR_DISCORD_BOT_TOKEN') {
      errors.push('discord.token contains placeholder value');
    } else if (!validateDiscordToken(config.discord.token)) {
      warnings.push('discord.token format looks suspicious');
    }
  }

  // Validate Feishu
  if (feishuEnabled) {
    if (!config.feishu?.app_id) {
      errors.push('feishu.app_id is required when feishu is enabled');
    } else if (config.feishu.app_id === 'YOUR_FEISHU_APP_ID') {
      errors.push('feishu.app_id contains placeholder value');
    }
    if (!config.feishu?.app_secret) {
      errors.push('feishu.app_secret is required when feishu is enabled');
    } else if (config.feishu.app_secret === 'YOUR_FEISHU_APP_SECRET') {
      errors.push('feishu.app_secret contains placeholder value');
    }
  }

  // Check admins
  if (!config.admins || !Array.isArray(config.admins) || config.admins.length === 0) {
    warnings.push('No admins configured - you will not be able to manage the bot');
  }

  // Check whitelist
  if (config.whitelist?.enabled && (!config.whitelist.users || config.whitelist.users.length === 0)) {
    warnings.push('whitelist is enabled but no users are whitelisted');
  }

  return { errors, warnings };
}

/**
 * Validate config file
 */
export function validateConfigFile(configPath) {
  try {
    const config = parseConfig(configPath);
    return validateConfig(config);
  } catch (error) {
    return {
      errors: [error.message],
      warnings: []
    };
  }
}
