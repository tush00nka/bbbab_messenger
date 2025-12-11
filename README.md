# Quick start
1. Create a `.env` file or set the environment variables: `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`, `SERVER_PORT`
2. Build and run using `docker-compose`: 

```bash
docker-compose up
```

## Go Build Cache
The project uses Docker volumes to cache Go compilation artifacts between container rebuilds, significantly speeding up subsequent builds:
- `go-build-cache`: Stores compiled packages and build cache
- `go-mod-cache`: Stores downloaded Go modules

These volumes are automatically created and managed by Docker Compose. To clear the cache if needed run this:
```bash
docker volume rm bbbab_messenger_go-build-cache bbbab_messenger_go-mod-cache
```

# Docs
Swagger docs are available at /swagger
