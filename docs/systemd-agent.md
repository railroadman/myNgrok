# Running the agent with systemd

Install the Linux release binary as `/usr/local/bin/tunnel-agent`, then create
a dedicated unprivileged account and working directory:

```sh
sudo useradd --system --home /var/lib/tunnel-agent --shell /usr/sbin/nologin tunnel-agent
sudo install -d -o tunnel-agent -g tunnel-agent /var/lib/tunnel-agent
```

Copy [`deploy/systemd/tunnel-agent.service`](../deploy/systemd/tunnel-agent.service)
to `/etc/systemd/system/tunnel-agent.service`. Create
`/etc/myngrok/tunnel-agent.env` with mode `0600` and this content:

```text
LOCAL_ADDRESS=127.0.0.1:8080
GATEWAY_URL=wss://tunnel.example.com/api/v1/agent/connect
AGENT_TOKEN=tkn_replace_with_an_agent_token
```

The token is a credential: keep the environment file owned by root and do not
put it in a unit file, shell history, source control, or command output. Start
the service after installing the file:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now tunnel-agent
sudo systemctl status tunnel-agent
sudo journalctl -u tunnel-agent -f
```

The unit restarts the agent after a process failure. The agent itself also
reconnects to the gateway on transient WebSocket failures. To rotate a token,
revoke the old one in the control plane, change the environment file, then run
`sudo systemctl restart tunnel-agent`.
