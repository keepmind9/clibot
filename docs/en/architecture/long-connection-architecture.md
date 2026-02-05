# Long Connection Architecture for clibot

**Version**: v1.0
**Date**: 2026-01-29
**Status**: Architecture Update

---

## 1. Overview

clibot uses **long connection architecture** (WebSocket/long polling) for all Bot adapters. This eliminates the requirement for public IP addresses and enables deployment in any network environment.

---

## 2. Architecture Comparison

### 2.1 Traditional Webhook Approach (NOT USED)

```
User Message
    ↓
Discord/Feishu/Telegram Platform
    ↓ HTTP Webhook (requires public IP)
    ↓
clibot Server (must have public IP:8080)
```

**Problems:**
- ❌ Requires public IP address
- ❌ Needs port forwarding/router configuration
- ❌ Exposes service to internet (security risk)
- ❌ Cannot deploy in home/office networks behind NAT

### 2.2 Long Connection Approach (ADOPTED) ✅

```
clibot Server (any network)
    ↓ WebSocket / Long Polling
    ↓
Discord/Feishu/Telegram Platform
    ↓
User Message (received via connection)
```

**Advantages:**
- ✅ No public IP required
- ✅ Works in NAT networks (home, office)
- ✅ More secure (no exposed ports)
- ✅ Simpler deployment
- ✅ True "anywhere, anytime" access

---

## 3. Platform-Specific Implementations

### 3.1 Discord - WebSocket Gateway

**Technology**: WebSocket Gateway API
**Library**: `github.com/bwmarrin/discordgo`
**Connection Type**: Persistent WebSocket connection

**Implementation**:
```go
// Discord already uses WebSocket by default
session, err := discordgo.New("Bot " + token)
if err != nil {
    return fmt.Errorf("failed to create session: %w", err)
}

// Register message handler
session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
    // Message received via WebSocket
    messageHandler(BotMessage{
        Platform:  "discord",
        UserID:    m.Author.ID,
        Channel:   m.ChannelID,
        Content:   m.Content,
        Timestamp: m.Timestamp,
    })
})

// Open WebSocket connection
session.Open()
```

**How it Works**:
1. Bot connects to Discord Gateway WebSocket server
2. Maintains persistent connection
3. Receives real-time message events via WebSocket
4. Sends messages via HTTP REST API

**Advantages**:
- Official library support
- Real-time bidirectional communication
- Automatic reconnection handling
- No public IP needed

---

### 3.2 Telegram - Long Polling

**Technology**: Bot API long polling
**Library**: `github.com/go-telegram-bot-api/telegram-bot-api/v6` or custom HTTP client
**Connection Type**: HTTP long polling

**Implementation**:
```go
// Telegram long polling implementation
type TelegramBot struct {
    token   string
    client  *http.Client
    baseURL string
}

func (t *TelegramBot) Start(messageHandler func(BotMessage)) error {
    offset := 0
    for {
        // Long polling getUpdates API
        url := fmt.Sprintf("%s/bot%s/getUpdates?offset=%d&timeout=30",
            t.baseURL, t.token, offset)

        resp, err := t.client.Get(url)
        if err != nil {
            time.Sleep(5 * time.Second) // Wait before retry
            continue
        }

        var updates []TelegramUpdate
        json.Unmarshal(resp, &updates)

        for _, update := range updates {
            messageHandler(BotMessage{
                Platform:  "telegram",
                UserID:    fmt.Sprintf("%d", update.Message.From.ID),
                Channel:   fmt.Sprintf("%d", update.Message.Chat.ID),
                Content:   update.Message.Text,
                Timestamp: update.Message.Date,
            })
            offset = update.UpdateID + 1
        }
    }
}

func (t *TelegramBot) SendMessage(channel, message string) error {
    url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)

    payload := map[string]interface{}{
        "chat_id": channel,
        "text":   message,
    }

    body, _ := json.Marshal(payload)
    req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")

    resp, err := t.client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send message: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("telegram API error: %d", resp.StatusCode)
    }

    return nil
}
```

**How it Works**:
1. Bot calls `getUpdates` API with `timeout=30`
2. Server holds connection open for up to 30 seconds
3. When message arrives, returns immediately
4. Bot processes and polls again
5. Send messages via HTTP POST to `sendMessage` API

**Advantages**:
- Simple HTTP-based implementation
- No WebSocket library needed
- Works through any HTTP proxy
- No public IP needed

---

### 3.3 Feishu - Lark Open API

**Status**: ⚠️ **UNDER INVESTIGATION**

**Options**:
1. **Lark Event Subscription** - May require webhook (not ideal)
2. **Long polling API** - Check if available
3. **Internal deployment** - If user has Feishu enterprise instance

**Workaround (if long connection not supported)**:
- Use ngrok/frp for tunneling
- Deploy clibot in Feishu's network (VPC)
- Or skip Feishu initially, support Discord/Telegram first

**Recommendation**:
- Start with Discord + Telegram (both support long connection)
- Investigate Feishu API for long polling support
- Consider adding Feishu later if deployment needs arise

---

## 4. BotAdapter Interface (Updated)

```go
// BotAdapter interface - Updated with long connection architecture
type BotAdapter interface {
    // Start establishes long connection and starts listening for messages
    // - Discord: WebSocket Gateway connection
    // - Telegram: HTTP long polling loop
    // - Feishu: TBD (investigating)
    //
    // The connection is maintained from clibot to the platform,
    // eliminating the need for public IP addresses.
    Start(messageHandler func(BotMessage)) error

    // SendMessage sends a message to the platform
    // - Uses HTTP REST API for most platforms
    // - Works asynchronously
    SendMessage(channel, message string) error

    // Stop gracefully closes the connection and cleans up resources
    // - Closes WebSocket for Discord
    // - Stops polling loop for Telegram
    Stop() error
}
```

---

## 5. Deployment Scenarios

### 5.1 Home Network (NAT)

**Old (Webhook)**: ❌ Not possible (no public IP)
**New (Long Connection)**: ✅ Works perfectly

```
Home Network (192.168.1.x)
    └─ clibot Server
        └─ WebSocket → Discord Gateway ✅
        └─ Long Polling → Telegram API ✅
```

### 5.2 Office Network (Behind Firewall)

**Old (Webhook)**: ❌ Not possible (firewall blocks inbound)
**New (Long Connection)**: ✅ Works perfectly

```
Office Network (10.0.0.x)
    └─ clibot Server
        └─ WebSocket → Discord Gateway ✅
        └─ Long Polling → Telegram API ✅
```

### 5.3 Cloud VPS (With Public IP)

**Old (Webhook)**: ✅ Works but requires setup
**New (Long Connection)**: ✅ Works (no configuration needed)

```
Cloud VPS
    └─ clibot Server
        └─ WebSocket → Discord Gateway ✅
        └─ Long Polling → Telegram API ✅
```

---

## 6. Technical Benefits

### 6.1 Security

**No Exposed Ports**:
- No need to open port 8080
- No need to configure firewall
- Reduces attack surface

**Internal Network**:
- Bot can run in trusted network
- No direct internet exposure
- Easier to secure

### 6.2 Reliability

**Connection Management**:
- Automatic reconnection (Discord)
- Retry logic (Telegram polling)
- Connection state monitoring

**Failure Recovery**:
- Detect connection drops
- Automatic reconnection
- Graceful degradation

### 6.3 Simplicity

**Deployment**:
- No port forwarding
- No DNS configuration
- No SSL certificates needed (outbound connections only)

**Development**:
- Test locally without public IP
- Debug in real network environment
- Simpler CI/CD

---

## 7. Implementation Priority

### Phase 1: Core Platforms (MVP)
1. ✅ **Discord** - Already implemented with WebSocket
2. ⏳ **Telegram** - Implement long polling adapter

### Phase 2: Additional Platforms
3. ⏸️ **Feishu** - Investigate long polling support
   - If supported: Implement long polling
   - If not supported: Document workaround (ngrok, VPC deployment)
4. ⏸️ **Other platforms** - Evaluate based on user needs

---

## 8. Configuration Changes

No changes needed to existing `config.yaml` structure. Bot configuration remains the same:

```yaml
bots:
  discord:
    enabled: true
    token: "${DISCORD_TOKEN}"
    channel_id: "123456789"

  telegram:
    enabled: true
    token: "${TELEGRAM_TOKEN}"
    chat_id: "123456789"
```

The difference is **how the bot connects**, not **how it's configured**.

---

## 9. Migration Notes

### For Current Implementations

**Discord Bot (Task 4)**:
- ✅ Already using WebSocket via `discordgo`
- ✅ No changes needed
- Just document that it's long connection, not webhook

**Future Telegram Bot**:
- Implement long polling from the start
- Use `getUpdates` API with timeout
- Send messages via `sendMessage` API

---

## 10. Summary

**Key Decision**: clibot uses **long connection architecture** for all Bot adapters.

**Benefits**:
- ✅ No public IP required
- ✅ Works in any network (home, office, cloud)
- ✅ More secure (no exposed ports)
- ✅ Simpler deployment
- ✅ Better aligns with "anywhere, anytime" goal

**Implementation Status**:
- Discord: ✅ Already uses WebSocket (no changes)
- Telegram: ⏳ To be implemented with long polling
- Feishu: ⏸️ Investigating (may need workaround)

**This architectural decision makes clibot truly accessible from any network environment.**
