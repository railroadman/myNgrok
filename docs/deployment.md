# Production deployment

The development stack is started with `docker compose up --build`. For a public
deployment, put Caddy in front of the `backend` and `frontend` services and use
[`deploy/caddy/Caddyfile`](../deploy/caddy/Caddyfile).

Set these values in the Caddy container:

```text
CADDY_EMAIL=ops@example.com
PUBLIC_BASE_DOMAIN=tunnel.example.com
WILDCARD_TLS_CERT_FILE=/etc/caddy/certs/wildcard.crt
WILDCARD_TLS_KEY_FILE=/etc/caddy/certs/wildcard.key
```

Mount the configured Caddyfile at `/etc/caddy/Caddyfile:ro`, and mount the
certificate and private key at the paths above as read-only files. Persist
`/data` and `/config` as Docker volumes so Caddy can retain the root-domain
certificate and renewal state.

The root domain serves the Vue control plane. Caddy proxies `/api/`, health,
and metrics requests to the Go backend; all other root-domain requests go to
the frontend. Every `*.tunnel.example.com` host is proxied unchanged to the
backend, which selects the tunnel by its `Host` header.

## Wildcard DNS and TLS

Create both records at your DNS provider:

```text
tunnel.example.com     A/AAAA  <gateway public address>
*.tunnel.example.com   A/AAAA  <gateway public address>
```

If the provider supports it, a wildcard `CNAME` to a stable gateway hostname is
also valid. Verify that public DNS resolves a random name such as
`check.tunnel.example.com` before opening tunnels.

Wildcard certificates cannot be obtained with the normal HTTP-01 challenge.
Either issue one through your DNS provider or run a Caddy build with the
provider's DNS-challenge module. The supplied Caddyfile intentionally requires
an explicit wildcard certificate, while Caddy obtains the root-domain
certificate automatically.

Do not expose PostgreSQL or the backend's port directly to the internet. Give
the public host a firewall rule for ports 80 and 443 only, use unique production
secrets, and set `CORS_ALLOWED_ORIGINS` to the HTTPS control-plane origin.
