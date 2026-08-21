# Docker Compose for RADIUS Accounting Server

Quick start:

1. Build and start services:

```bash
docker-compose up --build
```

2. Check subscriber logs (mounted to `./logs/radius_updates.log`):

```bash
tail -f logs/radius_updates.log
```

Notes:
- Redis is started with `redis/redis.conf` which enables `notify-keyspace-events KEA`.
- Server listens on UDP port `1813`; compose maps host `1813:1813/udp`.
- Subscriber writes to `/var/log/radius_updates.log` inside the container; mapped to `./logs` on host.
