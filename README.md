# mclo.gs Uploader Bot

## Overview

This is a Discord bot, written in Go, that listens for uploaded log files, uploads them to mclo.gs (a popular Minecraft log analysis service), and posts the link back to the channel for easy reading.

[Why?](/why.md)

## Usage

```yaml
services:
  bot:
    image: ghcr.io/emilyxfox/go-mclogs-bot:latest
    restart: on-failure:3
    environment:
      DISCORD_TOKEN: ${DISCORD_TOKEN}
```

|Env var      |Description|Required/Default|
|-------------|-----------|----------------|
|DISCORD_TOKEN| A Discord bot token generated from [discord.dev](https://discord.dev/) | Required