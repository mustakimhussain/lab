# Self-Hosted Service - Monitor

A lightweight Go application designed to monitor the availability of local self-hosted services. It performs periodic TCP connectivity checks, tracks uptime history, and persists state to **Upstash Redis** (serverless Redis).

## Features

- **Real-time Health Checks**: Periodically pings configured services via TCP.
- **Uptime Tracking**: Maintains a `lastUpTime` timestamp to distinguish between "currently down" and "down since last seen."
- **Redis Persistence**: Uses Upstash REST API to store and retrieve service state, ensuring data survives restarts.
- **Extensible**: Easily add new services to the configuration list.

## Prerequisites

- **Go 1.20+** installed locally.
- An **Upstash Redis** instance with REST API enabled.
- **Credentials**:
  - `Upstash RESTURL`: Your Upstash database URL.
  - `Upstash RESTToken`: Your Upstash REST API Bearer token.

## Configuration

Edit the constants in `service-monitor.go`:

```go
const (
    UpstashRESTURL   = "https://your-instance-id.upstash.io"
    UpstashRESTToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
)
```

Update the `targets` slice in `main()` to match your local services:

```go
targets := []ServiceConfig{
    {ID: "pihole", Name: "Pi-hole", Address: "192.168.1.10:80"},
    {ID: "jenkins", Name: "Jenkins", Address: "192.168.1.10:8080"},
    // Add more services here
}
```

## Data Structure

The application pushes a `HeartbeatPayload` to Redis under the key `home:network:health`:

```json
{
  "timestamp": "2026-08-18T12:00:00Z",
  "services": {
    "pihole": {
      "name": "Pi-hole",
      "status": "up",
      "lastSeen": "2026-08-18T12:00:00Z",
      "lastUpTime": "2026-08-18T11:55:00Z",
      "error": ""
    },
    "jenkins": {
      "name": "Jenkins",
      "status": "down",
      "lastSeen": "2026-08-18T12:00:00Z",
      "lastUpTime": "2026-08-17T09:30:00Z",
      "error": "dial tcp [target-service]: connect: connection refused"
    }
  }
}
```
