# Garaj

A super simple self-hostable IPFS CAR upload service.

## Features

- Randomly generated API key stored in-memory
- Very smol (8.1MB binary size)
- Only upload CAR files, nothing else
- File size limit support (default 32MB)

## Install

```
go install github.com/talentlessguy/garaj@latest
```

## Usage

```sh
garaj -addr=":8080" -max-body-mb=4 -nodeaddr=":5001"
```