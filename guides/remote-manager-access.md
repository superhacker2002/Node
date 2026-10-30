# Access DeNode Manager Web remotely

By default, DeNode Manager is available only on the computer where it runs. To open it from another device, use an SSH tunnel or a private VPN.

**Keep Manager's TCP port 1111 private.** Manager uses HTTP, so opening this port to the internet can expose the interface and its traffic. SSH or a VPN provides a private connection.

The Datakeeper node uses a separate port for peer communication. Do not close that port or place it behind the VPN when configuring remote Manager access.

Choose a method:

- **SSH tunnel:** use it if you have SSH access to the computer running Manager.
- **Private VPN:** use it to connect from a phone or another device. If you need a VPN, see the [VPN setup guide](./private-vpn-setup.md).

## Option 1: SSH tunnel

On the client computer, run:

```sh
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:1111:127.0.0.1:1111 SSH_USER@MANAGER_SSH_HOST
```

Replace `SSH_USER` and `MANAGER_SSH_HOST` with the credentials and address you normally use to SSH into the **Manager computer**. For a custom SSH port add `-p SSH_PORT`. Verify the host-key fingerprint through a trusted channel on first connection.

Keep the terminal open and open [http://localhost:1111](http://localhost:1111) on the same client computer. Press `Ctrl+C` to disconnect.

## Option 2: VPN

### 1. Find the Manager address

Connect the client to the VPN. Choose the IP address **assigned to the Manager computer** and reachable from the client:

| Your setup | Address to use | Example |
| --- | --- | --- |
| VPN server and Manager on the same computer | Manager computer's VPN address | `10.90.90.1` |
| VPN server is separate; Manager joins as a VPN client | Manager computer's own VPN address | `10.90.90.2` |
| VPN gateway routes to Manager's local network | Manager computer's reachable local address | `192.168.1.20` |

Find the addresses on the Manager computer with `ip -br address` (Linux), `ifconfig` (macOS), or `ipconfig` (Windows). Use the IP without `/24` or another prefix. Do not use the phone's address or a separate VPN server's address.

For a separate VPN server, routing and firewalls must permit traffic in both directions. If the VPN ends at a gateway, the connection between the gateway and Manager is outside the VPN encryption.

### 2. Allow access privately

Allow TCP **1111** from trusted VPN clients to the Manager computer. Remove public access to this port in the computer firewall, cloud firewall and router, including IPv6 where applicable. Keep your administrative connection open while changing firewall rules.

### 3. Start Manager on that address

On the Manager computer, stop Manager using your normal procedure. Make sure the interface with the selected address is active. Then run, replacing `10.90.90.2` with your Manager address:

```sh
cd ~/.denode-manager
./server --add-listen 10.90.90.2:1111
```

Alternatively, start or restart it using the management script:

```sh
DENODE_MANAGER_ADD_LISTEN=10.90.90.2:1111 bash denode-manager.sh
```

Use your usual installation directory and account. If Manager runs as a system service, add the flag or environment variable to the service configuration so it applies on restart. The installed version must support `--add-listen`. By default, Manager also remains available locally at `127.0.0.1:1111`.

### 4. Open Manager

On the VPN-connected client, open `http://10.90.90.2:1111`, replacing the IP with the Manager address you chose. On the Manager computer itself, use `http://localhost:1111`.

## Installing as a PWA

A VPN does not turn HTTP into HTTPS. The remote HTTP address can open in a browser, but an installable PWA generally needs HTTPS with a certificate trusted by the client and must meet the app's installability requirements. A home-screen shortcut may not be an installed PWA. This guide does not configure HTTPS. See [MDN's PWA installation requirements](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Making_PWAs_installable).

An SSH tunnel opened on the client provides a `localhost` URL, which browsers treat as secure for local development.

## Troubleshooting

- **No VPN connection:** check the VPN endpoint, UDP port, keys and firewalls. Generate traffic and check for a recent WireGuard handshake.
- **VPN connects but Manager does not open:** check the route to the Manager address, the Manager listener and the TCP 1111 firewall rule. With a separate gateway, check the return route too.
- **Manager fails to start:** confirm the selected address belongs to an active interface on the Manager computer and your version supports `--add-listen`.
- **SSH fails:** check SSH access and forwarding. Do not expose port 1111 to fix it.

To check for accidental public access, test each public IPv4 and IPv6 address from outside the VPN and local network, for example `nc -vz -w 3 MANAGER_PUBLIC_IP 1111`. Replace the placeholder with the public IP of the Manager computer or its router. A successful connection means the port is exposed; a failed test alone does not prove all paths are closed.

For the separate tunnel used by the Datakeeper **node**, see the [SSH Tunnel Guide](./ssh-tunnel.md).
