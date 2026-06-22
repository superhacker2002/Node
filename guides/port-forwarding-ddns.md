# Port Forwarding & Dynamic DNS (DDNS) Guide

> **Advanced setup.** These are advanced settings for making your node reachable. Routers, operating systems, and providers differ, so the exact menus and output on your machine may not match the examples here. Use this as a reference. We can't provide individual support for differences specific to your setup.

When your node's port is reachable from the internet, other nodes can connect to you directly, which makes your node faster and more reliable. This guide shows how to open (forward) that port on your router, confirm it works, and give your connection a permanent name with Dynamic DNS (DDNS) so your address stays the same even when your IP changes.

> 💡 **Why does this matter?** If you're unsure what a public IP is or why your node needs one, read the **[Public IP guide](./public-ip.md)** first — it explains the benefits and how to get a public IP from your provider.

Many home connections already have what they need, so this often works on the first try.

> Throughout this guide we use port `55050` (the node's default) as the example. Replace it with the port your node actually uses.

## Table of Contents
- [Before You Start: Can You Forward a Port?](#before-you-start-can-you-forward-a-port)
- [Step 1: Find Your Computer's Local IP](#step-1-find-your-computers-local-ip)
- [Step 2: Reserve That Local IP](#step-2-reserve-that-local-ip)
- [Step 3: Open the Port](#step-3-open-the-port)
- [Step 4: Allow the Port in Your Firewall](#step-4-allow-the-port-in-your-firewall)
- [Step 5: Check the Port From Outside](#step-5-check-the-port-from-outside)
- [Step 6: Keep a Permanent Address with DDNS](#step-6-keep-a-permanent-address-with-ddns)
- [Good to Know](#good-to-know)

## Before You Start: Can You Forward a Port?

Port forwarding works when your connection is reachable from the internet. A quick check tells you whether yours is.

1. **Turn off any VPN** (and Tailscale or proxies); otherwise the checks show the VPN's address, not your real one.
2. **Find your public IP:** open [whatismyipaddress.com](https://whatismyipaddress.com) in a browser (any device).
3. **Find your router's WAN IP:** log in to your router (usually `192.168.0.1` or `192.168.1.1`) and open the **Status / Internet / WAN** page.
4. **Compare the two:**
    - They **match** → you're directly reachable. ✅
    - The router's WAN IP is private (`10.x`, `192.168.x`, `172.16–31.x`) or in the `100.64.x` range → see the notes below.

> 💡 **CGNAT.** If your router's WAN IP is in the `100.64.x`–`100.127.x` range, your provider shares one address among many customers (Carrier-Grade NAT), so there's no address of your own to forward to. You have two options: ask your provider for a public ("white") IP, which many offer (sometimes for a small fee), or use the **[SSH Tunnel guide](./ssh-tunnel.md)** to make your node reachable another way.

> 💡 **Double NAT.** If the router's WAN IP is private (`192.168.x` / `10.x`) but **not** `100.64.x`, there's likely a second router (often your provider's modem) in front of yours. Either switch the upstream modem to **bridge mode**, or add the same forwarding rule on **both** devices.

## Step 1: Find Your Computer's Local IP

This is your computer's address inside your home network (it looks like `192.168.x.x` or `10.x.x.x`). Your router needs it to know where to send incoming traffic. Use the adapter you're actually connected with (Wi-Fi or Ethernet).

- **Windows:** run `ipconfig` and read the **IPv4 Address** line of your active adapter. Note the **Default Gateway** too; that's your router's address. *(GUI: Settings → Network & Internet → your connection → Properties.)*
- **macOS:** run `ipconfig getifaddr en0` for Wi-Fi (or `en1` for Ethernet). *(GUI: System Settings → Network → your connection → Details → TCP/IP.)*
- **Linux:** run `hostname -I` and take the `192.168.x.x` / `10.x.x.x` value.

## Step 2: Reserve That Local IP

Routers hand out local addresses automatically, so your computer could get a different one later, which would point the forwarding rule at the wrong place. Reserving the address keeps it fixed.

The easiest way is in the router: open **DHCP** (sometimes **LAN → Address Reservation** or **Static Leases**), find your computer in the list of connected devices, and bind it to its current IP by MAC address.

> 💡 You can instead set a static IP in your OS network settings, but that means entering the subnet mask, gateway, and DNS by hand. The router reservation is simpler and less error-prone.

## Step 3: Open the Port

Two ways: automatically with UPnP, or manually on the router. Pick whichever is easier.

### Option A: Automatically with UPnP

If UPnP is enabled on your router (usually under **NAT Forwarding → UPnP** or **Advanced**), a small tool can open the port without touching router menus:

- **Linux:** `sudo apt install miniupnpc`, then `upnpc -a 192.168.0.25 55050 55050 TCP` (use your local IP from Step 1).
- **Windows / macOS:** miniupnpc binaries and GUI tools are available.
- Check the mapping was created: `upnpc -l`.

> 💡 UPnP is convenient, but while it's on, any program on your network can open ports by itself. If you'd rather avoid that, turn it off and use the manual method.

### Option B: Manually on the Router

> Router menus differ between models, so there's no single set of clicks. Here's what to fill in; for exact steps on your model, check [portforward.com](https://portforward.com) or search "your_router_model port forwarding".

1. Log in to your router's admin page (the **Default Gateway** address from Step 1, usually `192.168.1.1`).
2. Find **Port Forwarding** (also called **Virtual Server** or **NAT**).
3. Add a rule:
    - External port → `55050`
    - Internal port → `55050`
    - Internal IP → your computer's local IP from Step 1 (some routers let you pick it from a device list)
    - Protocol → `TCP` (if the only choice is TCP/UDP or "Both", that's fine too)
4. Make sure the rule is **enabled**, then save (restart the router if it asks).

## Step 4: Allow the Port in Your Firewall

Your computer's firewall must let the connection in.

- **Linux (ufw):** `sudo ufw allow 55050`. If `sudo ufw status` says **inactive**, nothing is blocking and you can skip this.
- **Windows:** PowerShell as Administrator: `New-NetFirewallRule -DisplayName "DeNode" -Direction Inbound -LocalPort 55050 -Protocol TCP -Action Allow`. *(GUI: Windows Defender Firewall → Advanced settings → Inbound Rules → New Rule → Port → TCP 55050 → Allow.)*
- **macOS:** the firewall is usually off by default. If it's on (System Settings → Network → Firewall), open **Options** and allow incoming connections for your node app, or just click **Allow** if macOS prompts you when the node first starts.

> Make sure your node listens on `0.0.0.0:55050` (all interfaces), not only `127.0.0.1`, otherwise traffic forwarded by the router won't reach it.

## Step 5: Check the Port From Outside

The node must be running for this check to succeed (something has to be listening). Then test it **from a different network**, such as a phone on mobile data, or use an online checker:

1. Open [canyouseeme.org](https://canyouseeme.org) (or [yougetsignal.com/tools/open-ports](https://www.yougetsignal.com/tools/open-ports/)).
2. Enter your public IP and port `55050`, then run the check.

**Open / Success** → your node is reachable from the internet. ✅

If it still shows **closed**, re-check in order: the forwarding rule, the reserved local IP, the firewall, and that the node is listening on `0.0.0.0:55050`. If everything is correct and it's still closed, your provider may block incoming connections. In that case, use the **[SSH Tunnel guide](./ssh-tunnel.md)**.

## Step 6: Keep a Permanent Address with DDNS

Home IPs often change over time. When that happens, your node's address goes stale. **Dynamic DNS (DDNS)** gives you a permanent name (like `mynode.duckdns.org`) that automatically follows your current IP, so your address never changes for others.

> 💡 **Simplest option:** many routers have a **built-in DDNS client** (look under **DDNS** / **Dynamic DNS** in the router). If yours supports DuckDNS (or No-IP / Dynu), set it up right there and skip the scripts below; the router will keep the name updated for you.

If your router doesn't have it, set it up on your computer with **DuckDNS** (free).

### Step 6.1: Get a name and token

1. Go to [duckdns.org](https://duckdns.org) and sign in (GitHub / Google / Reddit; no separate password needed).
2. In the **sub domain** field, type a name (for example `mynode`) and click **add domain** → you now have `mynode.duckdns.org`.
3. Copy your **token** (the long string at the top of the page).

### Step 6.2: Test it once

Open this link in a browser (with your name and token). It should display `OK`:
```
https://www.duckdns.org/update?domains=mynode&token=YOUR_TOKEN&ip=
```
The empty `ip=` means "use the address I'm connecting from", so DuckDNS records your current IP. (`OK` = success, `KO` = wrong domain or token.)

### Step 6.3: Keep it updated automatically

**Linux / macOS, with cron:**
```bash
mkdir -p ~/duckdns
echo 'curl -s "https://www.duckdns.org/update?domains=mynode&token=YOUR_TOKEN&ip=" >/dev/null' > ~/duckdns/duck.sh
chmod +x ~/duckdns/duck.sh
crontab -e
```
If `crontab -e` asks which editor, choose **nano** (easiest). Add this line (updates every 5 minutes) and save:
```
*/5 * * * * ~/duckdns/duck.sh
```

**Windows, with Task Scheduler:**
Create `C:\duckdns\duck.ps1`:
```powershell
Invoke-WebRequest -Uri "https://www.duckdns.org/update?domains=mynode&token=YOUR_TOKEN&ip=" -UseBasicParsing | Out-Null
```
Open **Task Scheduler** → **Create Task**:
- **General:** tick **Run whether user is logged on or not**.
- **Triggers** → **New** → **On a schedule** → **Repeat task every 5 minutes** for a duration of **Indefinitely**.
- **Actions** → **New** → **Start a program**: `powershell.exe`, arguments: `-File C:\duckdns\duck.ps1`.

### Step 6.4: Verify the name points to your IP

Easiest (any device, in a browser): open [dnschecker.org](https://dnschecker.org), enter `mynode.duckdns.org`, record type **A**, and it should show your current public IP.

Or in a terminal (`nslookup` is built into Windows, macOS, and Linux):
```bash
nslookup mynode.duckdns.org
```

### Step 6.5: Set your node's address

In your node configuration, set the connection address to:
```
mynode.duckdns.org:55050
```
Now, even when your IP changes, DuckDNS updates the name and your address stays the same for everyone.

> 💡 DuckDNS also supports IPv6. If you're reachable over IPv6, add `&ipv6=` to the update link to also record an AAAA record.

## Good to Know

A few practical notes:

- A forwarded port lets peers reach your node directly. That's the point, and directly reachable nodes do the most work in the network. Forward only the port your node needs, and keep your node updated.
- Peers will see your IP, the same way any directly reachable server is visible. That's normal for a public node.
- Node traffic runs over your home connection, so if your link is slow or has a data cap, keep that in mind.
- If you can't forward a port (CGNAT, provider blocking, no router access), the **SSH Tunnel guide** gives you a reachable address another way.
