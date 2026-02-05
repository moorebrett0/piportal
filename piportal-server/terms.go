package main

import (
	"fmt"
	"net/http"
)

func (h *Handler) handleTermsPage(w http.ResponseWriter, r *http.Request) {
	domain := h.config.BaseDomain
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Terms of Service &amp; Privacy Policy — PiPortal</title>
    <meta name="description" content="PiPortal Terms of Service and Privacy Policy.">
    <link rel="canonical" href="https://%[1]s/terms">
    <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, -apple-system, sans-serif; color: #1e293b; background: #fff; line-height: 1.7; }
        a { color: #0075ff; text-decoration: none; }
        a:hover { text-decoration: underline; }

        /* Nav */
        nav { padding: 16px 0; border-bottom: 1px solid #e2e8f0; }
        nav .inner { max-width: 760px; margin: 0 auto; padding: 0 24px; display: flex; align-items: center; justify-content: space-between; }
        nav .logo { display: flex; align-items: center; gap: 10px; font-weight: 700; font-size: 1.25rem; color: #0f172a; }
        nav .logo img { height: 32px; width: auto; }
        nav .nav-links { display: flex; align-items: center; gap: 24px; }
        nav .nav-links a { font-size: 0.9rem; color: #475569; font-weight: 500; }
        nav .nav-links a:hover { color: #0075ff; text-decoration: none; }

        /* Content */
        .container { max-width: 760px; margin: 0 auto; padding: 48px 24px 80px; }
        h1 { font-size: 2rem; font-weight: 800; color: #0f172a; margin-bottom: 4px; }
        .updated { font-size: 0.9rem; color: #64748b; font-style: italic; margin-bottom: 40px; }
        h2 { font-size: 1.25rem; font-weight: 700; color: #0f172a; margin-top: 40px; margin-bottom: 12px; padding-top: 20px; border-top: 1px solid #f1f5f9; }
        h2:first-of-type { border-top: none; padding-top: 0; }
        h3 { font-size: 1rem; font-weight: 600; color: #0f172a; margin-top: 24px; margin-bottom: 8px; }
        p { font-size: 0.95rem; color: #334155; margin-bottom: 12px; }
        ul, ol { margin-bottom: 12px; padding-left: 24px; }
        li { font-size: 0.95rem; color: #334155; margin-bottom: 6px; }
        .caps { text-transform: uppercase; font-size: 0.9rem; }
        .divider { border: none; border-top: 2px solid #e2e8f0; margin: 56px 0; }
        /* Footer */
        footer { padding: 32px 0; border-top: 1px solid #e2e8f0; text-align: center; }
        footer p { font-size: 0.85rem; color: #94a3b8; }
        footer a { color: #64748b; margin: 0 10px; font-size: 0.85rem; }
        footer a:hover { color: #0075ff; }

        @media (max-width: 640px) {
            h1 { font-size: 1.5rem; }
            .container { padding: 32px 20px 60px; }
        }
    </style>
</head>
<body>

<nav>
  <div class="inner">
    <a href="/" class="logo"><img src="/logo.png" alt="PiPortal">PiPortal</a>
    <div class="nav-links">
      <a href="/">Home</a>
      <a href="/status">Status</a>
      <a href="https://discord.gg/uuYtV5Ukk7" target="_blank" rel="noopener">Join the Beta</a>
    </div>
  </div>
</nav>

<div class="container">

<h1>Pi Portal Terms of Service</h1>
<p class="updated">Last Updated: February 4th, 2026</p>

<h2>1. Introduction</h2>
<p>These Terms of Service ("Terms") govern your use of Pi Portal ("Service"), operated by PiPortal ("we," "us," or "our"). By using our Service, you agree to these Terms.</p>

<h2>2. Description of Service</h2>
<p>Pi Portal provides fleet management tools for Raspberry Pi and similar devices, including:</p>
<ul>
  <li>Remote device monitoring and metrics</li>
  <li>Browser-based terminal access</li>
  <li>Device alerts and notifications</li>
  <li>Remote device actions (reboot, etc.)</li>
</ul>

<h2>3. Account Registration</h2>
<ul>
  <li>You must provide accurate and complete information when creating an account</li>
  <li>You are responsible for maintaining the security of your account credentials</li>
  <li>You must be at least 18 years old to use the Service</li>
  <li>One person or legal entity may not maintain more than one free account</li>
</ul>

<h2>4. Acceptable Use</h2>
<p>You agree NOT to use the Service to:</p>
<ul>
  <li>Violate any applicable laws or regulations</li>
  <li>Access devices you do not own or have permission to manage</li>
  <li>Distribute malware or malicious code</li>
  <li>Interfere with or disrupt the Service or servers</li>
  <li>Attempt to gain unauthorized access to any systems</li>
  <li>Use the Service for any illegal or harmful purpose</li>
  <li>Resell access to the Service without our permission</li>
</ul>

<h2>5. Device Agent Software</h2>
<ul>
  <li>Our agent software must only be installed on devices you own or have authorization to manage</li>
  <li>You are responsible for all activity conducted through devices registered to your account</li>
  <li>You must keep the agent software updated to the latest version</li>
  <li>We may push updates to the agent software to fix bugs or security issues</li>
</ul>

<h2>6. Data and Privacy</h2>
<ul>
  <li>We collect device metrics (CPU, memory, temperature, disk usage, online status) to provide the Service</li>
  <li>We do not access or store the contents of your SSH sessions</li>
  <li>Connection data is encrypted in transit using TLS</li>
  <li>See our <a href="#privacy">Privacy Policy</a> below for full details on data handling</li>
</ul>

<h2>7. Service Availability</h2>
<ul>
  <li>We strive for high availability but do not guarantee uninterrupted service</li>
  <li>We may perform maintenance that temporarily affects availability</li>
  <li>We are not liable for any downtime or service interruptions</li>
  <li>We reserve the right to modify or discontinue features with reasonable notice</li>
</ul>

<h2>8. Payment and Billing</h2>
<ul>
  <li>Paid plans are billed monthly or annually in advance</li>
  <li>Prices may change with 30 days notice</li>
  <li>No refunds for partial months of service</li>
  <li>Failure to pay may result in account suspension</li>
  <li>Free tier limitations may change at any time</li>
</ul>

<h2>9. Cancellation</h2>
<ul>
  <li>You may cancel your account at any time from your account settings</li>
  <li>Upon cancellation, your access continues until the end of your billing period</li>
  <li>We may retain certain data as required by law or for legitimate business purposes</li>
  <li>Device agents will stop functioning when your account is terminated</li>
</ul>

<h2>10. Intellectual Property</h2>
<ul>
  <li>The Service and its original content are owned by PiPortal</li>
  <li>You retain ownership of any data you transmit through the Service</li>
  <li>You grant us a limited license to process your data to provide the Service</li>
</ul>

<h2>11. Limitation of Liability</h2>
<p class="caps">To the maximum extent permitted by law:</p>
<ul>
  <li>The Service is provided "as is" without warranties of any kind</li>
  <li>We are not liable for any indirect, incidental, or consequential damages</li>
  <li>Our total liability shall not exceed the amount you paid us in the past 12 months</li>
  <li>We are not responsible for any damage to your devices or data loss</li>
</ul>

<h2>12. Indemnification</h2>
<p>You agree to indemnify and hold harmless PiPortal from any claims, damages, or expenses arising from:</p>
<ul>
  <li>Your use of the Service</li>
  <li>Your violation of these Terms</li>
  <li>Your violation of any third-party rights</li>
</ul>

<h2>13. Changes to Terms</h2>
<ul>
  <li>We may modify these Terms at any time</li>
  <li>Material changes will be communicated via email or through the Service</li>
  <li>Continued use after changes constitutes acceptance of new Terms</li>
</ul>

<h2>14. Termination</h2>
<p>We may terminate or suspend your account immediately, without notice, for:</p>
<ul>
  <li>Violation of these Terms</li>
  <li>Suspected fraudulent, abusive, or illegal activity</li>
  <li>Non-payment of fees</li>
  <li>Extended periods of inactivity</li>
</ul>

<h2>15. Governing Law</h2>
<p>These Terms are governed by the laws of the State of Oregon, United States, without regard to conflict of law provisions.</p>

<h2>16. Contact</h2>
<p>Questions about these Terms? Contact us at:</p>
<ul>
  <li>Email: <a href="mailto:brett@piportal.dev">brett@piportal.dev</a></li>
</ul>

<hr class="divider" id="privacy">

<h1>Privacy Policy</h1>
<p class="updated">Last Updated: February 4th, 2026</p>

<h2>Information We Collect</h2>

<h3>Account Information</h3>
<ul>
  <li>Email address</li>
  <li>Password (hashed, we cannot read it)</li>
  <li>Payment information (processed by our payment provider)</li>
</ul>

<h3>Device Information</h3>
<ul>
  <li>Device name and identifier</li>
  <li>IP address</li>
  <li>Operating system version</li>
  <li>CPU, memory, disk, and temperature metrics</li>
  <li>Online/offline status</li>
  <li>Last connection timestamp</li>
</ul>

<h3>Usage Information</h3>
<ul>
  <li>Feature usage and interactions</li>
  <li>Error logs and diagnostics</li>
</ul>

<h2>Information We Do NOT Collect</h2>
<ul>
  <li>Contents of your SSH sessions (end-to-end encrypted)</li>
  <li>Files on your devices</li>
  <li>Network traffic content or payloads (we never inspect, store, or log what your device sends or receives)</li>
  <li>Personal data stored on your devices</li>
  <li>Websites or services your device connects to</li>
</ul>

<h2>Network Metrics We DO Collect</h2>
<p>To provide bandwidth monitoring and usage stats, we collect:</p>
<ul>
  <li>Bandwidth usage (bytes sent/received)</li>
  <li>Connection status (online/offline)</li>
  <li>Session duration (e.g., how long an SSH session lasted)</li>
</ul>
<p><strong>To be clear:</strong> We measure <em>how much</em> data moves, not <em>what</em> the data contains. We are not a man-in-the-middle and cannot see your traffic.</p>

<h2>How We Use Your Information</h2>
<ul>
  <li>To provide and maintain the Service</li>
  <li>To notify you of alerts and important updates</li>
  <li>To detect and prevent abuse or security threats</li>
  <li>To improve the Service</li>
  <li>To communicate with you about your account</li>
</ul>

<h2>Data Storage and Security</h2>
<ul>
  <li>All data is encrypted in transit (TLS 1.3)</li>
  <li>Device tokens are stored hashed</li>
  <li>Data is stored in the US West Region (Oregon)</li>
  <li>We implement industry-standard security practices</li>
</ul>

<h2>Data Retention</h2>
<ul>
  <li>Account data is retained while your account is active</li>
  <li>Device metrics are retained for 90 days</li>
  <li>You may request deletion of your data at any time</li>
</ul>

<h2>Third-Party Services</h2>
<p>We use the following third-party services:</p>
<ul>
  <li>[Payment processor] for billing</li>
  <li>[Hosting provider] for infrastructure</li>
  <li>[Email provider] for transactional emails</li>
</ul>

<h2>Your Rights</h2>
<p>You have the right to:</p>
<ul>
  <li>Access your personal data</li>
  <li>Correct inaccurate data</li>
  <li>Request deletion of your data</li>
  <li>Export your data</li>
  <li>Opt out of marketing communications</li>
</ul>

<h2>Children's Privacy</h2>
<p>The Service is not intended for children under 18. We do not knowingly collect data from children.</p>

<h2>Changes to Privacy Policy</h2>
<p>We may update this policy and will notify you of material changes via email or through the Service.</p>

<h2>Contact</h2>
<p>Privacy questions? Contact us at <a href="mailto:brett@piportal.dev">brett@piportal.dev</a>.</p>

</div>

<footer>
  <p>&copy; 2025 PiPortal. All rights reserved.</p>
  <div><a href="/">Home</a><a href="/fleet">Fleet Management</a><a href="/status">Status</a></div>
</footer>

</body>
</html>`, domain)
}
