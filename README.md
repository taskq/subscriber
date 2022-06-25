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
  --detached \
  --rm \
  --name taskq-subscriber \
  ghcr.io/taskq/subscriber/subscriber:1.0.0
```

