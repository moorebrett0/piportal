package main

import (
	"fmt"
	"net/http"
)

func (h *Handler) handleFleetPage(w http.ResponseWriter, r *http.Request) {
	domain := h.config.BaseDomain
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Raspberry Pi Fleet Management — PiPortal</title>
    <meta name="description" content="Manage hundreds of Raspberry Pi devices from a single dashboard. Secure tunnels, live monitoring, remote shell, and fleet-wide visibility with no VPN or open ports.">
    <link rel="canonical" href="https://%[1]s/fleet">
    <meta property="og:title" content="Raspberry Pi Fleet Management — PiPortal">
    <meta property="og:description" content="Manage hundreds of Raspberry Pi devices from a single dashboard. Secure tunnels, live monitoring, remote shell, and fleet-wide visibility.">
    <meta property="og:type" content="website">
    <meta property="og:url" content="https://%[1]s/fleet">
    <script type="application/ld+json">
    {
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      "name": "PiPortal Fleet Management",
      "applicationCategory": "DeveloperApplication",
      "operatingSystem": "Linux",
      "description": "Raspberry Pi fleet management platform with secure tunnels, live monitoring, and remote shell access.",
      "url": "https://%[1]s/fleet",
      "offers": {
        "@type": "Offer",
        "price": "0",
        "priceCurrency": "USD"
      }
    }
    </script>
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, -apple-system, sans-serif; color: #1e293b; background: #fff; line-height: 1.6; }
        a { color: #0075ff; text-decoration: none; }
        .container { max-width: 1060px; margin: 0 auto; padding: 0 24px; }

        /* Nav */
        nav { padding: 16px 0; border-bottom: 1px solid #e2e8f0; }
        nav .container { display: flex; align-items: center; justify-content: space-between; }
        nav .logo { display: flex; align-items: center; gap: 10px; font-weight: 700; font-size: 1.25rem; color: #0f172a; }
        nav .logo img { height: 32px; width: auto; }
        nav .nav-links { display: flex; align-items: center; gap: 24px; }
        nav .nav-links a { font-size: 0.9rem; color: #475569; font-weight: 500; }
        nav .nav-links a:hover { color: #0075ff; }
        .nav-cta { background: #0075ff; color: #fff !important; padding: 8px 20px; border-radius: 6px; font-size: 0.875rem; font-weight: 500; }
        .nav-cta:hover { background: #0060d0; }

        /* Hero */
        .hero { padding: 80px 0 64px; text-align: center; background: linear-gradient(180deg, #f0f7ff 0%%, #fff 100%%); }
        .hero .badge { display: inline-block; background: #eef6ff; color: #0075ff; font-size: 0.8rem; font-weight: 600; padding: 6px 14px; border-radius: 20px; margin-bottom: 20px; letter-spacing: 0.02em; }
        .hero h1 { font-size: 2.75rem; font-weight: 800; line-height: 1.15; color: #0f172a; max-width: 780px; margin: 0 auto 20px; }
        .hero h1 span { color: #0075ff; }
        .hero p { font-size: 1.2rem; color: #475569; max-width: 620px; margin: 0 auto 36px; }
        .hero-ctas { display: flex; justify-content: center; gap: 12px; flex-wrap: wrap; }
        .btn { display: inline-block; background: #0075ff; color: #fff; padding: 14px 32px; border-radius: 8px; font-size: 1rem; font-weight: 600; }
        .btn:hover { background: #0060d0; }
        .btn-outline { display: inline-block; border: 1px solid #cbd5e1; color: #334155; padding: 14px 32px; border-radius: 8px; font-size: 1rem; font-weight: 600; background: #fff; }
        .btn-outline:hover { border-color: #0075ff; color: #0075ff; }

        /* Section shared */
        section { padding: 72px 0; }
        .section-label { text-align: center; font-size: 0.8rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.08em; color: #0075ff; margin-bottom: 8px; }
        .section-title { text-align: center; font-size: 2rem; font-weight: 700; color: #0f172a; margin-bottom: 16px; }
        .section-subtitle { text-align: center; font-size: 1.05rem; color: #64748b; max-width: 640px; margin: 0 auto 48px; }
        .alt-bg { background: #f8fafc; }

        /* Problem / Pain points */
        .problems { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; max-width: 960px; margin: 0 auto; }
        .problem-card { display: flex; gap: 14px; padding: 20px; border: 1px solid #fee2e2; border-radius: 12px; background: #fff; }
        .problem-icon { flex-shrink: 0; width: 40px; height: 40px; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.2rem; background: #fef2f2; }
        .problem-card h3 { font-size: 0.95rem; font-weight: 600; margin-bottom: 4px; color: #0f172a; }
        .problem-card p { font-size: 0.85rem; color: #64748b; }

        /* Capabilities grid */
        .capabilities { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 24px; }
        .cap-card { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px 24px; }
        .cap-card .icon { width: 48px; height: 48px; background: #eef6ff; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-size: 1.4rem; margin-bottom: 16px; }
        .cap-card h3 { font-size: 1.1rem; font-weight: 600; margin-bottom: 8px; }
        .cap-card p { font-size: 0.9rem; color: #64748b; margin-bottom: 12px; }
        .cap-card ul { list-style: none; }
        .cap-card ul li { font-size: 0.85rem; color: #475569; padding: 3px 0 3px 20px; position: relative; }
        .cap-card ul li::before { content: "\2713"; position: absolute; left: 0; color: #0075ff; font-weight: 700; }

        /* How it works */
        .flow { display: grid; grid-template-columns: repeat(4, 1fr); gap: 24px; max-width: 960px; margin: 0 auto; position: relative; }
        .flow::before { content: ''; position: absolute; top: 32px; left: 10%%; right: 10%%; height: 2px; background: #e2e8f0; z-index: 0; }
        .flow-step { text-align: center; position: relative; z-index: 1; }
        .flow-num { display: inline-flex; align-items: center; justify-content: center; width: 48px; height: 48px; border-radius: 50%%; background: #0075ff; color: #fff; font-weight: 700; font-size: 1.2rem; margin-bottom: 16px; box-shadow: 0 0 0 6px #fff; }
        .flow-step h3 { font-size: 1rem; font-weight: 600; margin-bottom: 6px; }
        .flow-step p { font-size: 0.85rem; color: #64748b; }
        .flow-step code { display: block; margin-top: 8px; background: #0f172a; color: #e2e8f0; padding: 8px 10px; border-radius: 6px; font-size: 0.78rem; text-align: left; white-space: pre; overflow-x: auto; font-family: ui-monospace, monospace; }

        /* Scale numbers */
        .scale-banner { display: flex; justify-content: center; flex-wrap: wrap; gap: 56px; padding: 20px 0 40px; }
        .scale-stat { text-align: center; }
        .scale-stat .num { font-size: 2.5rem; font-weight: 800; color: #0075ff; line-height: 1.1; }
        .scale-stat .label { font-size: 0.85rem; color: #64748b; margin-top: 4px; }

        /* Use cases */
        .usecases { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 24px; }
        .usecase { border: 1px solid #e2e8f0; border-radius: 12px; padding: 28px 24px; background: #fff; }
        .usecase h3 { font-size: 1.1rem; font-weight: 600; margin-bottom: 8px; }
        .usecase p { font-size: 0.9rem; color: #64748b; margin-bottom: 10px; }
        .usecase .tag { display: inline-block; font-size: 0.75rem; background: #eef6ff; color: #0075ff; padding: 3px 10px; border-radius: 12px; font-weight: 600; }

        /* Security */
        .sec-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; max-width: 960px; margin: 0 auto; }
        .sec-item { display: flex; gap: 14px; padding: 20px; border: 1px solid #e2e8f0; border-radius: 10px; background: #fff; }
        .sec-icon { flex-shrink: 0; width: 40px; height: 40px; background: #eef6ff; border-radius: 10px; display: flex; align-items: center; justify-content: center; color: #0075ff; font-size: 1.2rem; }
        .sec-item h3 { font-size: 0.95rem; font-weight: 600; margin-bottom: 4px; }
        .sec-item p { font-size: 0.85rem; color: #64748b; }

        /* Comparison */
        .compare { max-width: 760px; margin: 0 auto; border: 1px solid #e2e8f0; border-radius: 12px; overflow: hidden; }
        .compare table { width: 100%%; border-collapse: collapse; }
        .compare th, .compare td { padding: 14px 20px; text-align: left; font-size: 0.9rem; }
        .compare thead th { background: #f8fafc; font-weight: 600; color: #334155; border-bottom: 1px solid #e2e8f0; }
        .compare thead th:first-child { color: #64748b; font-weight: 500; }
        .compare thead th:last-child { color: #0075ff; }
        .compare tbody td { border-bottom: 1px solid #f1f5f9; color: #475569; }
        .compare tbody tr:last-child td { border-bottom: none; }
        .compare .yes { color: #16a34a; font-weight: 600; }
        .compare .no { color: #94a3b8; }
        .compare .highlight { background: rgba(0, 117, 255, 0.03); }

        /* CTA */
        .cta-banner { text-align: center; padding: 80px 0; background: linear-gradient(180deg, #f0f7ff 0%%, #fff 100%%); }
        .cta-banner h2 { font-size: 2rem; font-weight: 700; color: #0f172a; margin-bottom: 12px; }
        .cta-banner p { font-size: 1.1rem; color: #64748b; max-width: 520px; margin: 0 auto 32px; }

        /* Footer */
        footer { padding: 48px 0 32px; border-top: 1px solid #e2e8f0; }
        .footer-inner { display: flex; justify-content: space-between; align-items: center; }
        .footer-inner p { color: #94a3b8; font-size: 0.85rem; }
        .footer-links { display: flex; gap: 20px; }
        .footer-links a { color: #64748b; font-size: 0.85rem; }
        .footer-links a:hover { color: #0075ff; }

        @media (max-width: 768px) {
            .hero h1 { font-size: 1.85rem; }
            .hero p { font-size: 1rem; }
            section { padding: 48px 0; }
            .section-title { font-size: 1.5rem; }
            .flow { grid-template-columns: 1fr 1fr; row-gap: 32px; }
            .flow::before { display: none; }
            .scale-banner { gap: 24px; }
            .scale-banner .num { font-size: 1.8rem; }
            .footer-inner { flex-direction: column; gap: 16px; text-align: center; }
        }
        @media (max-width: 480px) {
            .flow { grid-template-columns: 1fr; }
            .hero-ctas { flex-direction: column; align-items: center; }
        }
    </style>
</head>
<body>

<nav>
  <div class="container">
    <a href="/" class="logo"><img src="/logo.png" alt="PiPortal">PiPortal</a>
    <div class="nav-links">
      <a href="/">Home</a>
      <a href="/status">Status</a>
      <a href="https://discord.gg/uuYtV5Ukk7" class="nav-cta" target="_blank" rel="noopener">Join the Beta</a>
    </div>
  </div>
</nav>

<!-- Hero -->
<section class="hero">
  <div class="container">
    <div class="badge">Fleet Management</div>
    <h1>One Dashboard for Your Entire <span>Raspberry Pi</span> Fleet</h1>
    <p>Deploy, monitor, and manage tens to hundreds of Raspberry Pi devices from a single pane of glass. No VPN, no open ports, no manual SSH sessions.</p>
    <div class="hero-ctas">
      <a href="https://discord.gg/uuYtV5Ukk7" class="btn" target="_blank" rel="noopener">Join the Beta</a>
      <a href="#how-it-works" class="btn-outline">See How It Works</a>
    </div>
  </div>
</section>

<!-- Scale numbers -->
<section>
  <div class="container">
    <div class="scale-banner">
      <div class="scale-stat"><div class="num">30s</div><div class="label">Per-device setup</div></div>
      <div class="scale-stat"><div class="num">0</div><div class="label">Open ports required</div></div>
      <div class="scale-stat"><div class="num">24/7</div><div class="label">Live monitoring</div></div>
      <div class="scale-stat"><div class="num">1</div><div class="label">Dashboard for all devices</div></div>
    </div>
  </div>
</section>

<!-- Pain Points -->
<section class="alt-bg">
  <div class="container">
    <div class="section-label">The Problem</div>
    <h2 class="section-title">Fleet Management Shouldn't Be This Hard</h2>
    <p class="section-subtitle">If you've managed more than a handful of Pis, you know the pain.</p>
    <div class="problems">
      <div class="problem-card">
        <div class="problem-icon">&#128274;</div>
        <div>
          <h3>SSH requires network access</h3>
          <p>Every Pi needs a static IP, VPN config, or port forwarding rule. One misconfigured firewall and you're locked out.</p>
        </div>
      </div>
      <div class="problem-card">
        <div class="problem-icon">&#128065;</div>
        <div>
          <h3>No visibility at scale</h3>
          <p>You can't tell which devices are online, overheating, or running out of disk until something breaks.</p>
        </div>
      </div>
      <div class="problem-card">
        <div class="problem-icon">&#128736;</div>
        <div>
          <h3>Manual device-by-device ops</h3>
          <p>Rebooting, checking logs, and debugging means opening individual SSH sessions to each device, one at a time.</p>
        </div>
      </div>
      <div class="problem-card">
        <div class="problem-icon">&#128737;</div>
        <div>
          <h3>Security is an afterthought</h3>
          <p>Open SSH ports, shared credentials, and unencrypted connections are the norm. Attackers scan for exactly this.</p>
        </div>
      </div>
    </div>
  </div>
</section>

<!-- Capabilities -->
<section>
  <div class="container">
    <div class="section-label">The Solution</div>
    <h2 class="section-title">Everything You Need to Run a Pi Fleet</h2>
    <p class="section-subtitle">PiPortal gives you centralized visibility and control over every device, without the infrastructure overhead.</p>
    <div class="capabilities">
      <div class="cap-card">
        <div class="icon">&#128421;</div>
        <h3>Fleet Dashboard</h3>
        <p>See every device at a glance: online/offline status, CPU temperature, memory, disk, and uptime in a real-time grid.</p>
        <ul>
          <li>Instant online/offline detection</li>
          <li>Per-device system metrics</li>
          <li>Search and filter your fleet</li>
        </ul>
      </div>
      <div class="cap-card">
        <div class="icon">&#128187;</div>
        <h3>Remote Terminal</h3>
        <p>Open a browser-based shell on any device with one click. No SSH keys, no VPN, no port forwarding. Just type.</p>
        <ul>
          <li>Full interactive PTY in the browser</li>
          <li>Works through NAT and firewalls</li>
          <li>Session auto-cleanup on disconnect</li>
        </ul>
      </div>
      <div class="cap-card">
        <div class="icon">&#128268;</div>
        <h3>Secure HTTPS Tunnels</h3>
        <p>Expose any local port on a Pi via a unique HTTPS subdomain. Share web apps, APIs, or dashboards with a link.</p>
        <ul>
          <li>Automatic TLS certificates</li>
          <li>Per-device subdomains</li>
          <li>Toggle forwarding on/off instantly</li>
        </ul>
      </div>
      <div class="cap-card">
        <div class="icon">&#128200;</div>
        <h3>Live Monitoring</h3>
        <p>CPU temperature, memory pressure, disk space, load average, and uptime stream to your dashboard continuously.</p>
        <ul>
          <li>Temperature alerts (color-coded)</li>
          <li>Memory and disk usage tracking</li>
          <li>Uptime and load visibility</li>
        </ul>
      </div>
      <div class="cap-card">
        <div class="icon">&#128260;</div>
        <h3>Remote Reboot</h3>
        <p>Restart any device from the dashboard with one click. The Pi agent reconnects automatically after reboot.</p>
        <ul>
          <li>One-click reboot from browser</li>
          <li>Auto-reconnect after restart</li>
          <li>No physical access needed</li>
        </ul>
      </div>
      <div class="cap-card">
        <div class="icon">&#128272;</div>
        <h3>Zero-Trust Security</h3>
        <p>Devices make outbound-only connections. No listening ports, no firewall holes, no attack surface on your network.</p>
        <ul>
          <li>Outbound-only connections</li>
          <li>TLS everywhere, no exceptions</li>
          <li>Token-based device auth</li>
        </ul>
      </div>
    </div>
  </div>
</section>

<!-- How It Works -->
<section class="alt-bg" id="how-it-works">
  <div class="container">
    <div class="section-label">How It Works</div>
    <h2 class="section-title">From Zero to Fleet in Minutes</h2>
    <p class="section-subtitle">Add a new device to your fleet in four steps. No agents to compile, no config files to write.</p>
    <div class="flow">
      <div class="flow-step">
        <div class="flow-num">1</div>
        <h3>Create Device</h3>
        <p>Add a device in the dashboard and get a unique token.</p>
      </div>
      <div class="flow-step">
        <div class="flow-num">2</div>
        <h3>Install Agent</h3>
        <p>One command installs the PiPortal agent on any Pi.</p>
        <code>curl -fsSL https://%[1]s/install.sh | bash</code>
      </div>
      <div class="flow-step">
        <div class="flow-num">3</div>
        <h3>Connect</h3>
        <p>Start the agent. It connects home automatically.</p>
        <code>piportal setup
piportal start --port 8080</code>
      </div>
      <div class="flow-step">
        <div class="flow-num">4</div>
        <h3>Manage</h3>
        <p>Monitor, shell in, reboot, and tunnel from the dashboard.</p>
      </div>
    </div>
  </div>
</section>

<!-- Use Cases -->
<section>
  <div class="container">
    <div class="section-label">Use Cases</div>
    <h2 class="section-title">Built for Real Deployments</h2>
    <p class="section-subtitle">From a handful of devices in a lab to hundreds in the field.</p>
    <div class="usecases">
      <div class="usecase">
        <h3>Retail &amp; Kiosk Networks</h3>
        <p>Manage Pi-powered digital signage, point-of-sale terminals, and kiosks across multiple locations. Update software, reboot hung devices, and monitor health without dispatching technicians.</p>
        <span class="tag">Multi-site</span>
      </div>
      <div class="usecase">
        <h3>IoT &amp; Sensor Networks</h3>
        <p>Deploy Pis as edge gateways in warehouses, farms, and factories. Monitor environmental data, check device health, and access local dashboards through secure tunnels.</p>
        <span class="tag">Edge computing</span>
      </div>
      <div class="usecase">
        <h3>Education &amp; Labs</h3>
        <p>Give students and researchers remote access to a shared pool of Pis. Each device gets its own tunnel and monitoring. No need to be on the campus network.</p>
        <span class="tag">Remote access</span>
      </div>
      <div class="usecase">
        <h3>Home Lab Clusters</h3>
        <p>Run a Kubernetes cluster, media server farm, or home automation stack across multiple Pis. One dashboard to see what's up, what's hot, and what needs a reboot.</p>
        <span class="tag">Self-hosting</span>
      </div>
    </div>
  </div>
</section>

<!-- Comparison -->
<section class="alt-bg">
  <div class="container">
    <div class="section-label">Comparison</div>
    <h2 class="section-title">PiPortal vs. The Alternatives</h2>
    <p class="section-subtitle">You could stitch together SSH, VPNs, and monitoring scripts. Or you could use one tool.</p>
    <div class="compare">
      <table>
        <thead>
          <tr>
            <th>Capability</th>
            <th>SSH + VPN</th>
            <th>PiPortal</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Setup per device</td>
            <td class="no">Keys, config, firewall rules</td>
            <td class="yes highlight">One command</td>
          </tr>
          <tr>
            <td>Works behind NAT</td>
            <td class="no">Requires port forwarding or VPN</td>
            <td class="yes highlight">Yes, outbound-only</td>
          </tr>
          <tr>
            <td>Fleet dashboard</td>
            <td class="no">None (build your own)</td>
            <td class="yes highlight">Built-in</td>
          </tr>
          <tr>
            <td>Live metrics</td>
            <td class="no">Manual (ssh + scripts)</td>
            <td class="yes highlight">Automatic, real-time</td>
          </tr>
          <tr>
            <td>Browser terminal</td>
            <td class="no">Requires SSH client</td>
            <td class="yes highlight">One-click in dashboard</td>
          </tr>
          <tr>
            <td>HTTPS tunnels</td>
            <td class="no">ngrok / cloudflared per device</td>
            <td class="yes highlight">Built-in per device</td>
          </tr>
          <tr>
            <td>Remote reboot</td>
            <td class="no">SSH required</td>
            <td class="yes highlight">One click</td>
          </tr>
          <tr>
            <td>Open ports needed</td>
            <td class="no">SSH (22), VPN, etc.</td>
            <td class="yes highlight">Zero</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</section>

<!-- Security -->
<section>
  <div class="container">
    <div class="section-label">Security</div>
    <h2 class="section-title">Fleet Security by Default</h2>
    <p class="section-subtitle">Every device in your fleet gets the same security guarantees, automatically.</p>
    <div class="sec-grid">
      <div class="sec-item">
        <div class="sec-icon">&#128274;</div>
        <div><h3>Encrypted Connections</h3><p>All device-to-server and browser-to-server traffic uses TLS-encrypted WebSockets. Nothing travels in plain text.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#128737;</div>
        <div><h3>No Inbound Ports</h3><p>Devices only make outbound connections. Your network has zero additional attack surface from PiPortal.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#128273;</div>
        <div><h3>Per-Device Tokens</h3><p>Every device authenticates with a unique token. Revoke any device instantly from the dashboard without affecting others.</p></div>
      </div>
      <div class="sec-item">
        <div class="sec-icon">&#9881;</div>
        <div><h3>Tunnels Off by Default</h3><p>HTTP tunnel forwarding is disabled until you explicitly enable it. Local services stay unreachable until you say otherwise.</p></div>
      </div>
    </div>
  </div>
</section>

<!-- CTA -->
<section class="cta-banner">
  <div class="container">
    <h2>Start managing your Pi fleet today</h2>
    <p>PiPortal is in beta. Join the Discord to get early access, share feedback, and help shape the product.</p>
    <div class="hero-ctas">
      <a href="https://discord.gg/uuYtV5Ukk7" class="btn" target="_blank" rel="noopener">Join the Beta</a>
    </div>
  </div>
</section>

<footer>
  <div class="container">
    <div class="footer-inner">
      <p>&copy; 2025 PiPortal. All rights reserved.</p>
      <div class="footer-links">
        <a href="/">Home</a>
        <a href="/fleet">Fleet Management</a>
        <a href="/status">Status</a>
        <a href="https://discord.gg/uuYtV5Ukk7" target="_blank" rel="noopener">Join the Beta</a>
      </div>
    </div>
  </div>
</footer>

</body>
</html>`, domain)
}
