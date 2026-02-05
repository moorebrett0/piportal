import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api, type DeviceInfo, type OrgInfo } from '../api';
import StatusBadge from '../components/StatusBadge';
import BandwidthBar from '../components/BandwidthBar';
import Terminal from '../components/Terminal';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h ${mins}m`;
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m`;
}

function tempClass(temp: number): string {
  if (temp < 0) return '';
  if (temp < 60) return 'temp-ok';
  if (temp < 80) return 'temp-warm';
  return 'temp-hot';
}

export default function DeviceDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [device, setDevice] = useState<DeviceInfo | null>(null);
  const [orgs, setOrgs] = useState<OrgInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [rebooting, setRebooting] = useState(false);
  const [togglingTunnel, setTogglingTunnel] = useState(false);
  const [changingOrg, setChangingOrg] = useState(false);
  const [terminalOpen, setTerminalOpen] = useState(false);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      api.getDevice(id),
      api.listOrgs()
    ])
      .then(([deviceData, orgsData]) => {
        setDevice(deviceData);
        setOrgs(orgsData);
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
  }, [id]);

  const handleDelete = async () => {
    if (!device || !confirm(`Delete ${device.subdomain}? This cannot be undone.`)) return;
    setDeleting(true);
    try {
      await api.deleteDevice(device.id);
      navigate('/dashboard');
    } catch (err: any) {
      setError(err.message);
      setDeleting(false);
    }
  };

  const handleReboot = async () => {
    if (!device || !confirm(`Reboot ${device.subdomain}? The device will go offline briefly.`)) return;
    setRebooting(true);
    try {
      await api.rebootDevice(device.id);
    } catch (err: any) {
      setError(err.message);
    }
    setRebooting(false);
  };

  const handleToggleTunnel = async () => {
    if (!device) return;
    setTogglingTunnel(true);
    try {
      const res = await api.setTunnelEnabled(device.id, !device.tunnel_enabled);
      setDevice({ ...device, tunnel_enabled: res.tunnel_enabled });
    } catch (err: any) {
      setError(err.message);
    }
    setTogglingTunnel(false);
  };

  const handleOrgChange = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    if (!device) return;
    const newOrgId = e.target.value || null;
    setChangingOrg(true);
    try {
      await api.setDeviceOrg(device.id, newOrgId);
      const newOrgName = newOrgId ? orgs.find(o => o.id === newOrgId)?.name : undefined;
      setDevice({ ...device, org_id: newOrgId || undefined, org_name: newOrgName });
    } catch (err: any) {
      setError(err.message);
    }
    setChangingOrg(false);
  };

  if (loading) return <div className="loading">Loading...</div>;
  if (error) return <div className="error-msg">{error}</div>;
  if (!device) return <div className="error-msg">Device not found</div>;

  return (
    <div className="detail-page">
      <div className="page-header">
        <h1>{device.subdomain}</h1>
        <StatusBadge online={device.is_online} />
      </div>

      <div className="detail-grid">
        <div className="detail-section">
          <h2>Info</h2>
          <dl>
            <dt>URL</dt>
            <dd>
              {device.tunnel_enabled ? (
                <a href={device.url} target="_blank" rel="noopener">{device.url}</a>
              ) : (
                <span className="url-disabled">{device.url} <span className="url-disabled-note">(forwarding off)</span></span>
              )}
            </dd>
            <dt>Tier</dt>
            <dd>{device.tier}</dd>
            <dt>Tag</dt>
            <dd>
              <select
                value={device.org_id || ''}
                onChange={handleOrgChange}
                disabled={changingOrg}
                className="org-select"
              >
                <option value="">No tag</option>
                {orgs.map(org => (
                  <option key={org.id} value={org.id}>{org.name}</option>
                ))}
              </select>
            </dd>
            <dt>Created</dt>
            <dd>{new Date(device.created_at).toLocaleDateString()}</dd>
            {device.last_seen_at && (
              <>
                <dt>Last Seen</dt>
                <dd>{new Date(device.last_seen_at).toLocaleString()}</dd>
              </>
            )}
          </dl>
        </div>

        <div className="detail-section tunnel-toggle-section">
          <h2>Tunnel Forwarding</h2>
          <div className="tunnel-toggle-row">
            <div className="tunnel-toggle-status">
              <span className={`tunnel-state ${device.tunnel_enabled ? 'tunnel-on' : 'tunnel-off'}`}>
                {device.tunnel_enabled ? 'Enabled' : 'Disabled'}
              </span>
              <span className="tunnel-toggle-hint">
                {device.tunnel_enabled
                  ? 'HTTP requests to your subdomain are forwarded to your device.'
                  : 'HTTP forwarding is off. Visitors will see a 403 error.'}
              </span>
            </div>
            <button
              onClick={handleToggleTunnel}
              className={`btn ${device.tunnel_enabled ? 'btn-danger' : ''}`}
              disabled={togglingTunnel}
            >
              {togglingTunnel ? 'Updating...' : device.tunnel_enabled ? 'Disable Tunnel' : 'Enable Tunnel'}
            </button>
          </div>
        </div>

        {device.is_online && device.mem_total != null && (
          <div className="detail-section">
            <h2>System</h2>
            <div className="metrics-grid">
              {device.cpu_temp != null && device.cpu_temp >= 0 && (
                <div className="metric-item">
                  <div className={`metric-value ${tempClass(device.cpu_temp)}`}>
                    {device.cpu_temp.toFixed(1)}&deg;C
                  </div>
                  <div className="metric-label">CPU Temp</div>
                </div>
              )}
              <div className="metric-item">
                <div className="metric-value">
                  {formatBytes(device.mem_total! - device.mem_free!)} / {formatBytes(device.mem_total!)}
                </div>
                <div className="metric-label">Memory</div>
              </div>
              {device.disk_total != null && device.disk_total > 0 && (
                <div className="metric-item">
                  <div className="metric-value">
                    {formatBytes(device.disk_total! - device.disk_free!)} / {formatBytes(device.disk_total!)}
                  </div>
                  <div className="metric-label">Disk</div>
                </div>
              )}
              {device.uptime != null && device.uptime > 0 && (
                <div className="metric-item">
                  <div className="metric-value">{formatUptime(device.uptime)}</div>
                  <div className="metric-label">Uptime</div>
                </div>
              )}
              {device.load_avg != null && device.load_avg >= 0 && (
                <div className="metric-item">
                  <div className="metric-value">{device.load_avg.toFixed(2)}</div>
                  <div className="metric-label">Load Avg</div>
                </div>
              )}
            </div>
          </div>
        )}

        {device.is_online && (
          <div className="detail-section">
            <h2>Terminal</h2>
            {terminalOpen ? (
              <Terminal deviceId={device.id} onDisconnect={() => setTerminalOpen(false)} />
            ) : (
              <div className="terminal-connect-row">
                <span className="terminal-connect-hint">Open a shell session on this device.</span>
                <button className="btn" onClick={() => setTerminalOpen(true)}>Connect</button>
              </div>
            )}
          </div>
        )}

        <div className="detail-section">
          <h2>Bandwidth (This Month)</h2>
          <BandwidthBar used={device.bytes_total} limit={device.limit} />
        </div>

        <div className="detail-section">
          <h2>Setup</h2>
          <p>Run <code>piportal setup</code> on your Pi, then <code>piportal start --port 8080</code>.</p>
        </div>

        <div className="detail-section danger-zone">
          <h2>Danger Zone</h2>
          <div style={{ display: 'flex', gap: '12px' }}>
            {device.is_online && (
              <button onClick={handleReboot} className="btn btn-danger" disabled={rebooting}>
                {rebooting ? 'Rebooting...' : 'Reboot Device'}
              </button>
            )}
            <button onClick={handleDelete} className="btn btn-danger" disabled={deleting}>
              {deleting ? 'Deleting...' : 'Delete Device'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
