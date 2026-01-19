# Alerting

kubechronicle supports sending alerts to multiple channels when Kubernetes resources change. The alerting system is configurable and supports filtering by operation type.

## Supported Channels

- **Slack**: Send alerts to Slack via webhooks
- **Telegram**: Send alerts to Telegram via bot API
- **Email**: Send alerts via SMTP
- **Webhook**: Send alerts to custom webhook endpoints

## Configuration

Alerting is configured via the `ALERT_CONFIG` environment variable, which should contain a JSON object with the alert configuration.

### Configuration Structure

```json
{
  "slack": {
    "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
    "channel": "#alerts",
    "username": "kubechronicle"
  },
  "telegram": {
    "bot_token": "YOUR_BOT_TOKEN",
    "chat_ids": ["123456789", "-987654321"]
  },
  "email": {
    "smtp_host": "smtp.example.com",
    "smtp_port": 587,
    "smtp_username": "user@example.com",
    "smtp_password": "password",
    "from": "kubechronicle@example.com",
    "to": ["alerts@example.com"],
    "subject": "[kubechronicle] {{operation}}: {{resource}} in {{namespace}}"
  },
  "webhook": {
    "url": "https://your-webhook-endpoint.com/alerts",
    "method": "POST",
    "headers": {
      "Authorization": "Bearer YOUR_TOKEN",
      "X-Custom-Header": "value"
    }
  },
  "operations": ["CREATE", "UPDATE", "DELETE"]
}
```

### Operation Filtering

The `operations` field is optional. If specified, alerts will only be sent for the listed operations. If omitted or empty, alerts will be sent for all operations (CREATE, UPDATE, DELETE).

Example: To only alert on CREATE and DELETE operations:
```json
{
  "operations": ["CREATE", "DELETE"],
  "slack": { ... }
}
```

## Channel-Specific Configuration

### Slack

**Required**:
- `webhook_url`: Slack incoming webhook URL

**Optional**:
- `channel`: Channel override (defaults to webhook's configured channel)
- `username`: Bot username (defaults to webhook's configured name)

**Setup**:
1. Create a Slack app in your workspace
2. Enable "Incoming Webhooks"
3. Create a webhook and copy the URL
4. Add the URL to your configuration

### Telegram

**Required**:
- `bot_token`: Telegram bot token from @BotFather
- `chat_ids`: Array of chat IDs (can be user IDs or group IDs)

**Optional**: None

**Setup**:
1. Talk to @BotFather on Telegram
2. Create a new bot with `/newbot`
3. Copy the bot token
4. Get your chat ID (send a message to your bot, then visit `https://api.telegram.org/bot<TOKEN>/getUpdates`)
5. Add bot token and chat IDs to configuration

### Email

**Required**:
- `smtp_host`: SMTP server hostname
- `smtp_port`: SMTP server port (typically 25, 465, or 587)
- `from`: Sender email address
- `to`: Array of recipient email addresses

**Optional**:
- `smtp_username`: Username for SMTP authentication
- `smtp_password`: Password for SMTP authentication
- `subject`: Email subject template (supports `{{operation}}`, `{{resource}}`, `{{namespace}}`)

**Note**: If `smtp_username` and `smtp_password` are not provided, SMTP will attempt unauthenticated connection.

### Webhook

**Required**:
- `url`: Webhook endpoint URL

**Optional**:
- `method`: HTTP method (default: POST)
- `headers`: Map of custom headers to include in the request

**Payload**: The webhook receives the full `ChangeEvent` JSON object as the request body.

**Example Webhook Payload**:
```json
{
  "id": "CREATE-Deployment-test-app-1234567890",
  "timestamp": "2024-01-14T12:00:00Z",
  "operation": "CREATE",
  "resource_kind": "Deployment",
  "namespace": "default",
  "name": "test-app",
  "actor": {
    "username": "user@example.com",
    "groups": ["system:authenticated"],
    "service_account": "",
    "source_ip": "192.168.1.1"
  },
  "source": {
    "tool": "kubectl"
  },
  "diff": []
}
```

## Deployment Example

### Using Environment Variable (Kubernetes Secret)

1. Create a Kubernetes Secret with the alert configuration:

```bash
kubectl create secret generic kubechronicle-alert-config \
  --from-literal=ALERT_CONFIG='{
    "slack": {
      "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
    },
    "operations": ["CREATE", "DELETE"]
  }'
```

2. Update the deployment to use the secret:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubechronicle-webhook
spec:
  template:
    spec:
      containers:
      - name: webhook
        env:
        - name: ALERT_CONFIG
          valueFrom:
            secretKeyRef:
              name: kubechronicle-alert-config
              key: ALERT_CONFIG
```

### Using ConfigMap (for non-sensitive configuration)

For webhook URLs or other non-sensitive config, you can use ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubechronicle-alert-config
data:
  ALERT_CONFIG: |
    {
      "webhook": {
        "url": "https://your-webhook-endpoint.com/alerts"
      },
      "operations": ["CREATE", "UPDATE", "DELETE"]
    }
```

## Multiple Channels

You can configure multiple channels simultaneously. All configured channels will receive alerts:

```json
{
  "slack": {
    "webhook_url": "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
  },
  "email": {
    "smtp_host": "smtp.example.com",
    "smtp_port": 587,
    "from": "kubechronicle@example.com",
    "to": ["alerts@example.com"]
  },
  "webhook": {
    "url": "https://your-webhook-endpoint.com/alerts"
  }
}
```

## Behavior

- **Non-blocking**: Alert sending is asynchronous and does not block event processing
- **Fail-safe**: If alert sending fails, the error is logged but event processing continues
- **Filtering**: Operation filtering is applied before sending alerts
- **Formatting**: Each channel formats messages appropriately (Slack attachments, Telegram HTML, Email plain text, Webhook JSON)

## Troubleshooting

### Alerts not being sent

1. Check logs for alert sending errors
2. Verify configuration JSON is valid
3. Test webhook URLs/credentials manually
4. Check network connectivity from webhook pods

### Configuration not loading

1. Verify `ALERT_CONFIG` environment variable is set
2. Check JSON syntax is valid
3. Verify required fields are present for each channel

### Operation filtering not working

1. Verify `operations` array contains valid values: `["CREATE", "UPDATE", "DELETE"]`
2. Case-sensitive: use uppercase operation names
3. Empty array or missing field = all operations
