# Secure remote access to DeNode Manager Web

DeNode Manager Web uses TCP port **1111**. Do not expose it directly to the internet: the current HTTP connection does not encrypt session tokens. For remote access, use a private VPN or an SSH tunnel and restrict public access to the manager port.

Choose one of the two methods below:

- [Option A: VPN](#option-a-vpn) — access the manager through a private network, without opening an SSH tunnel each time.
- [Option B: SSH tunnel](#option-b-ssh-tunnel) — forward a local port through an encrypted SSH connection.

> This guide covers the **manager interface**. The Datakeeper node uses a separate port to communicate with peers. Do not close or change the node's port as part of these steps.

## Before you begin

You need access to the machine running the manager and permission to manage its firewall, cloud security group and any router port forwarding. If you do not manage these, ask the server administrator. Keep a working administrative connection open while changing firewall rules and confirm you can reconnect before disconnecting.

The current Manager Web may listen on `0.0.0.0:1111`, which makes it reachable on every network interface unless firewall rules restrict access. Both methods below can work with that setting; neither requires changing it to localhost.

## Option A: VPN

A VPN connects your computer and the manager server through an encrypted private network. The manager is accessed using the **server's VPN address**, while public access to TCP 1111 remains blocked.

### 1. Connect both devices to a private VPN

Use an existing trusted VPN, or set one up on both the server running Manager Web and the computer from which you manage it. Follow the instructions for your VPN to add both devices to the same private network.

Identify the server's VPN IP address or private VPN hostname in your VPN client or admin console. This is different from the server's public IP. Ensure both devices are connected and can reach one another over the VPN.

### 2. Restrict who can access the manager

In VPN access policies, allow only trusted users or devices to connect to TCP **1111** on the manager server. In the server firewall, allow TCP 1111 from the trusted VPN interface or VPN client addresses, and block public access to that port. Also remove public TCP 1111 rules and forwarding from any cloud firewall and router, for IPv4 and IPv6 where applicable.

Firewall rules are evaluated differently by different platforms. Verify the rule order and the VPN provider's routing/firewall requirements for your setup. Keep SSH or another administrative connection available while making changes. A general-purpose consumer VPN that merely routes internet traffic is not enough: both devices must be able to reach each other through the private VPN.

### 3. Open the manager over VPN

On your computer, connect to the VPN and enter the server's private VPN address in the browser:

```text
http://SERVER_VPN_IP:1111
```

Replace `SERVER_VPN_IP` with the actual VPN address. For an IPv6 address, use brackets: `http://[SERVER_VPN_IPV6]:1111`. Sign in to the manager as usual.

The connection uses HTTP inside the encrypted VPN tunnel. Restrict VPN access to trusted devices; VPN encryption does not protect against a compromised endpoint or untrusted device with access to the VPN. If Manager Web is bound only to `127.0.0.1`, another VPN device cannot access it directly: use an SSH tunnel or configure the manager to listen on a VPN-reachable interface.

Verify that access works over the VPN, then [verify that the public port is closed](#verify-public-access-is-blocked). For connection and access policy details, consult the documentation for your VPN.

## Option B: SSH tunnel

### 1. Restrict public access

In the server firewall, cloud security group and router, remove public inbound and forwarding rules for TCP **1111**, for IPv4 and IPv6. Keep SSH available from trusted addresses and confirm you can reconnect before closing your existing session.

### 2. Open a tunnel from your computer

Run this command in Terminal (macOS or Linux) or PowerShell (Windows). Replace `SSH_USER` and `SERVER_IP` with your SSH username and server address:

```sh
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:1111:127.0.0.1:1111 SSH_USER@SERVER_IP
```

If SSH uses a port other than 22, add `-p SSH_PORT` before `SSH_USER@SERVER_IP`. If you use an SSH key, add `-i PATH_TO_PRIVATE_KEY`. Verify the server host-key fingerprint through a trusted channel when connecting for the first time.

Keep this terminal open. The command normally shows no output once connected. It forwards port 1111 on your computer through SSH to port 1111 on the server.

### 3. Open the manager

On the same computer, open [http://localhost:1111](http://localhost:1111) and sign in. The HTTP connection is local to your computer; traffic between your computer and the server travels through SSH. When finished, sign out and stop the tunnel with `Ctrl+C`.

If local port 1111 is already in use, stop the conflicting local service and retry. The current web app expects `localhost:1111` when opened locally. If the tunnel fails, check SSH credentials, the SSH port and whether the server permits port forwarding; do not open public port 1111 to fix SSH.

## Verify public access is blocked

From a different machine **outside the server's network and VPN**, test each public IPv4 and IPv6 address as applicable. For example:

```sh
nc -vz -w 3 SERVER_IP 1111
```

A successful connection means the port is still reachable: check the server firewall, cloud rules and router forwarding. A timeout or refusal is consistent with the port being blocked, but check each public address and network path; one failed test does not prove every path is closed. This test does not invoke a manager API or delete an account.

If the manager is inaccessible over VPN, check that it is running and listening on an interface reachable over VPN, the VPN policy allows the connection, and the server firewall allows trusted VPN traffic. If it is inaccessible through SSH, check that the tunnel remains open and that the manager is listening on port 1111 on the server.

For the separate reverse tunnel used to expose a Datakeeper **node** port, see the [SSH Tunnel Guide](./ssh-tunnel.md).
