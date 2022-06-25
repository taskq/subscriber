# TaskQ Subscriber

# Configuration

```json
{
    "redis": {
        "channel": "junk"
    },
    "plugins": []
}
```

# Provisioning

## Docker/PodMan

```bash
podman run \
  --network host \
  --interactive \
  --tty \
  --detach \
  --rm \
  -v /opt/taskq/subscriber/subscriber.conf:/subscriber.conf \
  -e "REDIS_ADDRESS=10.32.0.238" \
  --name taskq-subscriber \
  ghcr.io/taskq/subscriber/subscriber:1.0.0
```

