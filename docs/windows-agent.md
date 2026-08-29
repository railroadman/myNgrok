# Running a tunnel agent from Windows

The agent runs directly as a Windows `.exe`. You do not need WSL, Docker, or a
Windows service to expose a local HTTP application.

## Local end-to-end check

1. Start the gateway stack from the repository root:

   ```powershell
   docker compose up --build
   ```

2. Open `http://localhost:5173`, register or sign in, then open **Manage agent
   tokens** and create a token. Copy it immediately: it is shown only once.

3. Start a local HTTP service. For example, in another PowerShell window:

   ```powershell
   python -m http.server 3000
   ```

4. Build the Windows agent and connect it to that service:

   ```powershell
   .\scripts\build-agent-release.ps1 -Version 0.1.0 -Target windows/amd64
   .\dist\agent\tunnel-agent_0.1.0_windows_amd64.exe http 3000 `
     --token <AGENT_TOKEN> `
     --gateway ws://localhost:8095/api/v1/agent/connect
   ```

   The agent prints `Connected to gateway` and its session ID. Keep this window
   open while the tunnel is needed.

5. In the web application, open **View tunnels**. Copy the displayed
   subdomain. With the local Compose configuration, test it at:

   ```text
   http://<subdomain>.tunnel.localhost:8095
   ```

   The local address uses HTTP because Caddy/TLS is not part of the development
   Compose stack. Docker maps host port `8095` to the gateway's internal port
   `8080`.

## Connect Windows to a deployed gateway

Use the same Windows executable, but point it to the public HTTPS gateway:

```powershell
.\dist\agent\tunnel-agent_0.1.0_windows_amd64.exe http 3000 `
  --token <AGENT_TOKEN> `
  --gateway wss://tunnel.example.com/api/v1/agent/connect
```

The public URL is `https://<subdomain>.tunnel.example.com` after wildcard DNS
and TLS are configured. See [deployment.md](deployment.md) for Caddy,
certificates, and DNS requirements.

## Development-only alternative

When Go is installed, run the current source without creating an executable:

```powershell
Set-Location agent
go run ./cmd/tunnel-agent http 3000 --token <AGENT_TOKEN> --gateway ws://localhost:8095/api/v1/agent/connect
```

Never put the token in a committed file. Revoke the token in the web UI if it
is exposed or the Windows computer is no longer trusted.
