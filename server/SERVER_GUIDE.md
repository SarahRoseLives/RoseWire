# RoseWire Server Setup Guide 🚀

Welcome to RoseWire! This guide will walk you through the initial setup of your new server instance and how to configure it to run as a system service on Linux.

---

## Part 1: First-Time Configuration

This part covers the initial steps to get your server running for the first time. These steps are crucial and include interactive prompts that only appear on the first run.

### Prerequisites

Before you begin, make sure you have the following:

- A compiled RoseWire server binary (e.g., `rosewire-server`).
- The two HTML files: `admin_login.html` and `admin_dashboard.html`.
- A domain name (e.g., `your-domain.com`) pointing to your server's public IP address.
- A valid TLS certificate (`fullchain.pem`) and private key (`privkey.pem`) for your domain.  
  *We recommend using Let's Encrypt with certbot.*
- Firewall ports opened for TCP traffic on ports **2222** (for SSH) and **8080** (for HTTPS).

---

### Step 1: Create a Directory and Place Files

It's best practice to run the server from a dedicated directory.

```bash
mkdir /opt/rosewire
cd /opt/rosewire
```

Place the `rosewire-server` binary, `admin_login.html`, and `admin_dashboard.html` files inside this new directory.

Copy your TLS certificate and key into this directory as well.  
They must be named `fullchain.pem` and `privkey.pem`.

```bash
# Example command, adjust the source path to your actual certificate location
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem /opt/rosewire/
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem /opt/rosewire/
```

---

### Step 2: Generate the SSH Host Key

The server requires an Ed25519 host key to operate the SSH service.

From within your `/opt/rosewire` directory, run:

```bash
ssh-keygen -t ed25519 -f server_ed25519 -N ""
```

This will generate two files: `server_ed25519` (the private key) and `server_ed25519.pub` (the public key).

---

### Step 3: Create the Configuration File

RoseWire uses a `config.json` file for its main settings. Create this file in the `/opt/rosewire` directory.

Create and edit `config.json`:

```bash
nano config.json
```

Copy and paste the template below, making sure to replace `YOURDOMAINHERE` with your actual domain.

```json
{
  "domain": "YOURDOMAINHERE",
  "ssh_listen_addr": "0.0.0.0:2222",
  "http_listen_addr": "0.0.0.0:8080",
  "cert_file": "fullchain.pem",
  "key_file": "privkey.pem",
  "peers": [
    "rosewire.rosevines.network:8080",
    "sarahsforge.dev:8080"
  ]
}
```

**Explanation of Fields:**

- `domain`: Required. Your public domain name.
- `ssh_listen_addr`: The IP and port for the SSH service.
- `http_listen_addr`: The IP and port for the web services (status page, admin, API). The server will provide HTTPS on this port.
- `cert_file` & `key_file`: Relative paths to your TLS certificate and key, which should be in the same directory.
- `peers`: A list of initial servers to connect with.

---

### Step 4: Initial Run & Admin Password Setup

You must run the server manually the first time to set up the administrator password.

Make the binary executable:

```bash
chmod +x rosewire-server
```

Run the server:

```bash
./rosewire-server
```

The server will detect that no admin password is set and will prompt you in the console:

```
--- Admin Password Setup ---
Please create a password for the admin web panel (user: SYSTEM):
```

Enter a secure password (it will not be visible) and press Enter. You will be asked to confirm it.  
This password is for the web-based admin panel, not for SSH.

Once the password is set, the server will create `admin.json` and `instance_key.pem` and continue starting up.  
You should see log output indicating that the HTTPS and SSH servers are listening.

---

### Step 5: Register the SYSTEM Admin User 🔑

With the server running manually, you must now register your administrative SSH key by connecting to the server using your RoseWire client application.

Launch your RoseWire app and configure a new connection with the following details:

- **Host:** `YOURDOMAINHERE` (the domain from your `config.json`)
- **Username:** `SYSTEM`

Connect using the app. The server will automatically register the SSH public key your client uses.

> **Important:**  
> The very first key to successfully connect as SYSTEM will be permanently registered as the administrator's key.  
> The server saves this association in the `nicks.db` file.  
> Ensure you are using the correct computer/client for this first connection.

After connecting, you are in the chat!  
You can verify your user is online by visiting the status page at `https://YOURDOMAINHERE:8080`.

You can now stop the manually running server by pressing **Ctrl+C** in its terminal window.

---

**Congratulations! Your RoseWire instance is now fully configured.**

---

## Part 2: Installing as a systemd Service ⚙️

To ensure your server runs automatically on boot and can be easily managed, you should set it up as a systemd service.

---

### Step 1: Create a Dedicated User

For security, it's best to run the service as a non-root user.

```bash
sudo useradd --system --user-group --shell /bin/false --home-dir /opt/rosewire rosewire
```

---

### Step 2: Set Permissions

The new `rosewire` user needs ownership of the application directory and all its contents.

```bash
sudo chown -R rosewire:rosewire /opt/rosewire
```

---

### Step 3: Create the systemd Service File

Create a new service file using a text editor:

```bash
sudo nano /etc/systemd/system/rosewire.service
```

Copy and paste the following simplified configuration.  
Remember to adjust the path if you installed RoseWire somewhere other than `/opt/rosewire`.

```ini
[Unit]
Description=RoseWire Federated Chat and File Sharing Server
After=network.target

[Service]
User=rosewire
Group=rosewire
WorkingDirectory=/opt/rosewire
ExecStart=/opt/rosewire/rosewire-server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

### Step 4: Enable and Start the Service

Now you can use the `systemctl` command to manage your new service.

Reload systemd to make it aware of the new service file:

```bash
sudo systemctl daemon-reload
```

Enable the service to start on boot:

```bash
sudo systemctl enable rosewire.service
```

Start the service immediately:

```bash
sudo systemctl start rosewire.service
```

---

### Step 5: Verify the Service Status ✅

You can check that the service is running correctly and view its logs.

Check the status:

```bash
sudo systemctl status rosewire.service
```

You should see an **active (running)** status in green.

View the logs in real-time:

```bash
sudo journalctl -u rosewire.service -f
```

---

Your RoseWire instance is now fully installed and managed by systemd!  
You can access the status page at `https://YOURDOMAINHERE:8080` and the admin panel at `https://YOURDOMAINHERE:8080/admin`.

---