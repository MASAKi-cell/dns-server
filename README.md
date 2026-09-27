## Overview

A DNS protocol implementation in Go. (DNS message parsing/encoding, DNS client, recursive resolver, authoritative server)

> **Warning**: This repository is created for **educational purposes**. It is not intended for production use or as an internet-facing server. Security measures (rate limiting, access control, DoS protection, etc.) are not implemented. Please use only for testing and learning in local environments.

### Features

- **DNS Message Processing** - RFC 1035 compliant wire format encoding/decoding
- **DNS Client** - UDP-based DNS query transmission
- **Recursive Resolver** - Iterative resolution from root servers to authoritative servers
- **Authoritative Server** - Responds to DNS queries using zone files
- **Zone File Parser** - RFC 1035 format zone file parsing

### Supported Record Types

- A (IPv4 address)
- AAAA (IPv6 address)
- NS (Name Server)
- CNAME (Canonical Name)
- MX (Mail Exchange)
- TXT (Text)
- SOA (Start of Authority)

## Usage

### selfdig - DNS Query Tool

```bash
# Query A record using default (Google DNS 8.8.8.8)
selfdig example.com

# Specify server
selfdig @1.1.1.1 example.com

# Specify record type
selfdig example.com AAAA
selfdig example.com MX
selfdig example.com NS
```

### resolved - Recursive DNS Resolver

A recursive resolver that performs name resolution by traversing from root servers to authoritative servers.

```bash
# Start on default port (5353)
resolved

# Specify port
resolved -addr :5353

# Query with selfdig
selfdig @127.0.0.1:5353 example.com
```

### authd - Authoritative DNS Server

A DNS server that loads zone files and returns authoritative responses.

```bash
# Start with zone file
authd -zone example.zone -addr :5353
```

Example zone file:

```
$ORIGIN example.com.
$TTL 3600

@       IN  SOA   ns1.example.com. admin.example.com. (
                  2024010101  ; Serial
                  3600        ; Refresh
                  1800        ; Retry
                  604800      ; Expire
                  86400       ; Minimum TTL
                  )

@       IN  NS    ns1.example.com.
@       IN  A     192.0.2.1
www     IN  A     192.0.2.2
mail    IN  MX    10 mail.example.com.
mail    IN  A     192.0.2.3
```

## Project Structure

```
dns/
├── message/    # DNS message encoding/decoding
├── client/     # DNS client
├── server/     # UDP-based DNS server
├── resolver/   # Recursive resolver (including cache and root hints)
├── zone/       # Zone file parser
├── cmd/
│   ├── selfdig/   # DNS query tool
│   ├── resolved/  # Recursive resolver daemon
│   └── authd/     # Authoritative server daemon
└── docs/       # Documentation
```
