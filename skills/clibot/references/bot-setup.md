# Bot Setup Guide

Detailed instructions for creating and configuring bots on each platform.

## Table of Contents

1. [Telegram Bot](#telegram-bot)
   - [Step 1: Create Bot via BotFather](#step-1-create-bot-via-botfather)
   - [Step 2: Configure Bot (Optional)](#step-2-configure-bot-optional)
   - [Step 3: Get Your User ID](#step-3-get-your-user-id)
   - [Step 4: Test Your Bot](#step-4-test-your-bot)
   - [Step 5: Configure in Clibot](#step-5-configure-in-clibot)
   - [Common Issues](#common-issues-1)
2. [Discord Bot](#discord-bot)
   - [Step 1: Create Discord Application](#step-1-create-discord-application)
   - [Step 2: Create Bot User](#step-2-create-bot-user)
   - [Step 3: Configure Bot Settings](#step-3-configure-bot-settings)
   - [Step 4: Get Bot Token](#step-4-get-bot-token)
   - [Step 5: Get Bot Invitation URL](#step-5-get-bot-invitation-url)
   - [Step 6: Get User and Guild IDs](#step-6-get-user-and-guild-ids)
   - [Step 7: Configure in Clibot](#step-7-configure-in-clibot)
   - [Step 8: Invite Bot to Server](#step-8-invite-bot-to-server)
   - [Common Issues](#common-issues-2)
3. [Feishu/Lark Bot](#feishulark-bot)
   - [Step 1–9](#step-1-create-feishu-open-platform-account)
   - [Common Issues](#common-issues-3)
4. [Testing Your Bot Setup](#testing-your-bot-setup)
5. [Security Best Practices](#security-best-practices)
6. [Advanced Configuration](#advanced-configuration)
7. [Need More Help?](#need-more-help)

---

## Telegram Bot

### Step 1: Create Bot via BotFather

1. Open Telegram and search for **@BotFather**
2. Start a chat with BotFather
3. Send the command `/newbot`
4. Follow the instructions:
   - Choose a name for your bot (e.g., "My Claude Bot")
   - Choose a username for your bot (must end in 'bot', e.g., "MyClaudeBot")
5. BotFather will respond with a token like:
   ```
   123456789:ABCdefGHIjklMNOpqrsTUVwxyz
   ```
6. **Copy and save this token** - you'll need it for clibot configuration

### Step 2: Configure Bot (Optional)

BotFather provides several useful commands:

#### Set Bot Description
```
/setdescription
```
Then choose your bot and provide a description (e.g., "Chat with Claude AI")

#### Set Bot Commands
```
/setcommands
```
Choose your bot and paste:
```
help - Show available commands
slist - List all sessions
suse - Switch to a session
sclose - Close a session
sstatus - Show session status
status - Show bot status
whoami - Show your user info
echo - Show your user ID
```

#### Set Privacy Mode
```
/setprivacy
```
Choose your bot and select **Disable** - this allows the bot to read all messages in groups (useful for group chats)

#### Set Bot Picture
```
/setuserpic
```
Choose your bot and upload a profile picture

### Step 3: Get Your User ID

1. Start a conversation with your bot
2. Send any message (e.g., "hello")
3. Use the `/echo` command (after clibot is running)
4. The bot will reply with your user ID
5. **Copy and save this ID** - you'll need it for the admin/whitelist configuration

### Step 4: Test Your Bot

Before using with clibot, verify your bot works:

```bash
curl https://api.telegram.org/bot<YOUR_TOKEN>/getMe
```

Should return JSON with bot information:
```json
{
  "ok": true,
  "result": {
    "id": 123456789,
    "is_bot": true,
    "first_name": "My Claude Bot",
    "username": "MyClaudeBot",
    "can_join_groups": true,
    "can_read_all_group_messages": false,
    "supports_inline_queries": false
  }
}
```

### Step 5: Configure in Clibot

Add to your `config.yaml`:

```yaml
telegram:
  enabled: true
  token: "123456789:ABCdefGHIjklMNOpqrsTUVwxyz"
  webhook_url: ""  # Leave empty for polling mode

admins:
  - YOUR_USER_ID  # From step 3
```

### Common Issues

**Bot doesn't respond:**
- Verify token is correct (no extra spaces)
- Check bot is not blocked by Telegram
- Ensure clibot service is running

**Can't find user ID:**
- Use `/echo` command after clibot is running
- Or use alternative: https://t.me/userinfobot

**Bot not in group chats:**
- Enable privacy mode disabled in BotFather
- Add bot to group manually
- Make bot admin if needed

---

## Discord Bot

### Step 1: Create Discord Application

1. Go to https://discord.com/developers/applications
2. Click **"New Application"**
3. Give it a name (e.g., "Claude Bot")
4. Click **"Create"**

### Step 2: Create Bot User

1. In the left sidebar, click **"Bot"**
2. Click **"Add Bot"**
3. Confirm by clicking **"Yes, do it!"**
4. Your bot is now created

### Step 3: Configure Bot Settings

#### Important: Enable Message Content Intent

1. Scroll down to **"Privileged Gateway Intents"**
2. Toggle **"Message Content Intent"** to ON
3. Click **"Save Changes"**

**Note:** This is required for the bot to read message content!

#### Optional: Change Bot Details

- **Bot Username**: Change display name
- **Bot Avatar**: Upload profile picture
- **About Me**: Add description

### Step 4: Get Bot Token

1. In the **"Bot"** section
2. Click **"Reset Token"** (if you see it)
3. Click **"Copy"** under **"Token"**
4. **Save this token securely** - you won't see it again!

Token format: 59+ characters, alphanumeric with dashes and underscores.

### Step 5: Get Bot Invitation URL

1. In the left sidebar, click **"OAuth2"** → **"URL Generator"**
2. Under **"Scopes"**, check **"bot"**
3. Under **"Bot Permissions"**, check:
   - Send Messages
   - Read Message History
   - Read Messages/View Channels
   - (Optional) Use Slash Commands
4. Copy the generated URL from the bottom
5. Paste in browser to invite bot to your server

### Step 6: Get User and Guild IDs

#### Get Your User ID:
1. Go to Discord
2. Go to Settings → Advanced
3. Enable **"Developer Mode"**
4. Right-click your name → **"Copy User ID"**

#### Get Guild (Server) ID:
1. Enable Developer Mode (if not already)
2. Right-click server name → **"Copy ID"**

### Step 7: Configure in Clibot

Add to your `config.yaml`:

```yaml
discord:
  enabled: true
  token: "YOUR_DISCORD_BOT_TOKEN_HERE"  # From step 4
  guild_id: "YOUR_GUILD_ID"  # From step 6

admins:
  - YOUR_USER_ID  # From step 6
```

### Step 8: Invite Bot to Server

1. Use the invitation URL from step 5
2. Select the server to add bot to
3. Authorize the bot
4. Complete the CAPTCHA if needed

### Common Issues

**Bot can't read messages:**
- Ensure "Message Content Intent" is enabled
- Check bot has "Read Messages" permission in server

**Bot not responding:**
- Verify token is correct (reset and copy again)
- Check bot is actually in the server
- Ensure clibot service is running

**Can't find Guild ID:**
- Enable Developer Mode in Discord
- Right-click server icon → Copy ID

---

## Feishu/Lark Bot

### Step 1: Create Feishu Open Platform Account

1. Go to https://open.feishu.cn/app
2. Log in with your Feishu account
3. If you don't have an account, click **"Register"**

### Step 2: Create Application

1. Click **"Create Application"**
2. Choose **"Create from Scratch"**
3. Fill in application details:
   - **App Name**: e.g., "Claude Bot"
   - **App Description**: e.g., "Chat with Claude AI"
   - **App Icon**: Upload optional icon
4. Click **"Create"**

### Step 3: Get App Credentials

1. In the left sidebar, click **"Credentials"**
2. You'll see:
   - **App ID**: Copy this
   - **App Secret**: Click **"Show"** and copy this
3. **Save both securely** - you'll need them for clibot

### Step 4: Configure Bot Permissions

1. In the left sidebar, click **"Permissions"** → **"Bot Permissions"**
2. Add required permissions:
   - **im:message** (Send messages)
   - **im:message:group_at_msg** (Read group messages)
   - **im:chat** (Access chat information)
3. Click **"Save"**

### Step 5: Activate Bot

1. In the left sidebar, click **"Bot"**
2. Toggle **"Enable Bot"** to ON
3. Fill in bot details:
   - **Bot Name**: e.g., "Claude Assistant"
   - **Bot Description**: e.g., "I'm here to help!"
   - **Bot Avatar**: Upload optional image
4. Click **"Save"**

### Step 6: Configure Event Subscriptions

1. In the left sidebar, click **"Event Subscriptions"**
2. Toggle **"Enable Events"** to ON
3. Subscribe to events:
   - **im.message.receive_v1** (Receive messages)
4. Configure Request URL (optional, for webhooks)
5. Click **"Save"**

**Note:** For development, you can leave Request URL empty and use polling mode.

### Step 7: Add Bot to Group Chat

1. Go to your Feishu app
2. Open or create a group chat
3. Click group settings → **"Add Bot"**
4. Search for your bot by name
5. Add bot to the group

### Step 8: Get User ID

1. In Feishu, click your profile picture
2. Your User ID will be displayed
3. **Copy and save this ID** - you'll need it for admin configuration

Alternatively:
- Send a message to the bot
- Use `/echo` command (after clibot is running)
- Bot will reply with your user ID

### Step 9: Configure in Clibot

Add to your `config.yaml`:

```yaml
feishu:
  enabled: true
  app_id: "cli_a1b2c3d4e5f6g7h8"  # From step 3
  app_secret: "your_app_secret_here"  # From step 3

admins:
  - YOUR_USER_ID  # From step 8
```

### Common Issues

**Bot not receiving messages:**
- Ensure bot is enabled
- Check bot is added to the group
- Verify permissions are granted
- Ensure event subscriptions are active

**Invalid app credentials:**
- Double-check App ID and App Secret
- Regenerate App Secret if needed
- Ensure no extra spaces in config

**Can't find user ID:**
- Use `/echo` command after clibot is running
- Check your profile in Feishu app
- Contact Feishu support if needed

---

## Testing Your Bot Setup

After configuring any bot, test it:

### 1. Start clibot
```bash
/clibot start
```

### 2. Send test message
Send a message to your bot:
- Telegram: Start a chat with your bot
- Discord: Mention your bot or send DM
- Feishu: Send message in group with bot

### 3. Use echo command
Send:
```
/echo
```

Bot should reply with your user information.

### 4. Try help command
Send:
```
/help
```

Bot should show available commands.

---

## Security Best Practices

### Protect Your Tokens

1. **Never commit tokens to git**
   ```bash
   # Add to .gitignore
   .clibot/config.yaml
   ```

2. **Use environment variables for sensitive data**
   ```yaml
   telegram:
     token: "${TELEGRAM_BOT_TOKEN}"
   ```

3. **Set proper file permissions**
   ```bash
   chmod 600 ~/.clibot/config.yaml
   ```

4. **Rotate tokens periodically**
   - Regenerate tokens every few months
   - Update config immediately after regeneration

### Limit Bot Access

1. **Enable whitelist mode** for private bots
2. **Add only trusted users** to admins
3. **Review access logs** regularly
4. **Remove inactive users** from whitelist

### Monitor Bot Activity

1. **Check logs regularly**
   ```bash
   tail -f ~/.clibot/logs/clibot.log
   ```

2. **Set up alerts** for unusual activity
3. **Rate limiting** to prevent abuse
4. **Audit sessions** periodically

---

## Advanced Configuration

### Webhook Mode (Production)

For production deployments, use webhooks instead of polling:

#### Telegram Webhook
```bash
curl -X POST "https://api.telegram.org/bot<TOKEN>/setWebhook" \
  -d "url=https://your-domain.com/webhook/telegram"
```

Config:
```yaml
telegram:
  enabled: true
  token: "YOUR_TOKEN"
  webhook_url: "https://your-domain.com/webhook/telegram"
```

#### Discord Webhook
Discord doesn't use traditional webhooks - it uses Gateway/HTTP API.

#### Feishu Webhook
```yaml
feishu:
  enabled: true
  app_id: "YOUR_APP_ID"
  app_secret: "YOUR_APP_SECRET"
  encrypt_key: "YOUR_ENCRYPT_KEY"  # For encrypted webhooks
  verification_token: "YOUR_VERIFICATION_TOKEN"
```

### Multiple Bot Instances

Run multiple bots on different platforms:

```yaml
telegram:
  enabled: true
  token: "TELEGRAM_TOKEN_1"

discord:
  enabled: true
  token: "DISCORD_TOKEN_1"

# Create another config file for additional bots
# clibot --config config2.yaml
```

---

## Need More Help?

- **Telegram Bot API**: https://core.telegram.org/bots/api
- **Discord Developer Portal**: https://discord.com/developers/docs
- **Feishu Open Platform**: https://open.feishu.cn/document
- **Clibot Issues**: https://github.com/keepmind9/clibot/issues
