# SSH Tunnel Guide

> **Advanced setup.** This is an advanced way to make your node reachable. Servers, operating systems, and shells differ, so the exact paths and output on your machine may not match the examples here. Use this as a reference. We can't provide individual support for differences specific to your setup.

If your node can't be reached directly from the internet (for example, you're behind CGNAT), you can make it reachable through a remote server that has a public IP, using an SSH reverse tunnel.

> 💡 **Alternative:** If you have direct internet access and can forward ports, the **[Port Forwarding & DDNS guide](guides/port-forwarding-ddns.md)** is simpler and doesn't require a remote server.

## How It Works

NAT lets outgoing connections through freely; only incoming ones are a problem. So a small tunnel client on your computer connects out to the server and keeps the tunnel open. The server accepts incoming connections from peers on its port and forwards them through the tunnel to your node.

```
peer → IP_VPS:55051 → [tunnel] → your home node :55051
```

The server acts as the "public face" of your node. Peers see `IP_VPS:55051`, but the connection actually reaches your computer behind NAT. All traffic flows through the server. That gives you reachability, but at scale the server's bandwidth is your ceiling.

> Throughout this guide, replace `IP_VPS` with your server's IP address. Port `55051` is an example; use your node's port.

## Table of Contents
- [Part A: On the Server (once)](#part-a-on-the-server-once)
- [Part B: Key for Password-Free Login](#part-b-key-for-password-free-login)
- [Part C: Start the Tunnel Manually (test)](#part-c-start-the-tunnel-manually-test)
- [Part D: Auto-Start (per OS)](#part-d-auto-start-per-os)
- [Part E: Node Address](#part-e-node-address)

## Part A: On the Server (once)

Connect to the server (as root):
```bash
ssh root@IP_VPS
```
Enable forwarding to the outside and restart the SSH server:
```bash
echo "GatewayPorts yes" >> /etc/ssh/sshd_config
systemctl restart ssh
ufw allow 55051        # if ufw is used on the server
```
> If `/etc/ssh/sshd_config` already has a `GatewayPorts` line (often a commented `#GatewayPorts no`), change it to `GatewayPorts yes` instead of adding a second one. sshd uses the first value.

Everything below is done on your home computer (where the node runs).

## Part B: Key for Password-Free Login

Auto-start can't type a password, so login to the server must use an SSH key.

**Linux / macOS:**
```bash
ssh-keygen -t ed25519
# when asked for a passphrase, just press Enter (no passphrase — required for auto-start)
ssh-copy-id root@IP_VPS
ssh root@IP_VPS "echo ok"      # must log in WITHOUT asking for a password
```

**Windows (PowerShell):**
```powershell
ssh-keygen -t ed25519
# press Enter through all questions, leave the passphrase empty
type $env:USERPROFILE\.ssh\id_ed25519.pub | ssh root@IP_VPS "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"
ssh root@IP_VPS "echo ok"      # must log in WITHOUT a password
```
If `echo ok` ran without a password prompt, the key works.

## Part C: Start the Tunnel Manually (test)

The command is the same on all operating systems:
```bash
ssh -N -R 0.0.0.0:55051:localhost:55051 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes root@IP_VPS
```
What it means: `-N` runs the tunnel only, with no remote commands. `-R 0.0.0.0:55051:localhost:55051` forwards port `55051` on the server to your local `55051`. The `ServerAlive*` and `ExitOnForwardFailure` options make a broken connection drop by itself, so auto-start can bring it back up.

The command "hangs" and prints nothing. That's normal: the tunnel is working. Check it from the outside, then press `Ctrl+C` and set up auto-start.

## Part D: Auto-Start (per OS)

### Linux: autossh + systemd
```bash
sudo apt install -y autossh
sudo nano /etc/systemd/system/ssh-tunnel.service
```
```ini
[Unit]
Description=SSH reverse tunnel
After=network-online.target
Wants=network-online.target

[Service]
User=YOUR_USER
ExecStart=/usr/bin/autossh -M 0 -N -R 0.0.0.0:55051:localhost:55051 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes root@IP_VPS
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
```
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ssh-tunnel
sudo systemctl status ssh-tunnel       # active (running)
```
> `User=YOUR_USER` matters: the service must run as the user who owns the SSH key from Part B.

### macOS: autossh + launchd
```bash
brew install autossh
which autossh        # note the path: usually /opt/homebrew/bin/autossh (M1/M2/M3) or /usr/local/bin/autossh (Intel)
```
Create `~/Library/LaunchAgents/com.denode.sshtunnel.plist` (use the autossh path from `which autossh`):
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.denode.sshtunnel</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/autossh</string>
    <string>-M</string><string>0</string>
    <string>-N</string>
    <string>-R</string><string>0.0.0.0:55051:localhost:55051</string>
    <string>-o</string><string>ServerAliveInterval=30</string>
    <string>-o</string><string>ServerAliveCountMax=3</string>
    <string>-o</string><string>ExitOnForwardFailure=yes</string>
    <string>root@IP_VPS</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict>
</plist>
```
```bash
launchctl load ~/Library/LaunchAgents/com.denode.sshtunnel.plist
```

### Windows: reconnect loop + startup
Windows has no autossh, so we use a simple self-restart. Create the file `C:\tunnel\ssh-tunnel.bat`:
```bat
:loop
ssh -N -R 0.0.0.0:55051:localhost:55051 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes root@IP_VPS
timeout /t 5
goto loop
```
When ssh drops, it waits 5 seconds and reconnects. No password is asked; it uses the key from Part B.

To start it automatically: press Win+R, type `shell:startup`, and put a shortcut to `ssh-tunnel.bat` there (it starts at login). For a background start with no window and before login, set it up through Task Scheduler with the "Run whether user is logged on or not" option.

## Part E: Node Address

In your node configuration, set the connection address to:
```
IP_VPS:55051
```

The chain: peer → `IP_VPS:55051` → SSH tunnel → your node at home. The address is permanent, kept alive by auto-start, and reconnects by itself if the connection drops.
