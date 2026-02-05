import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { api, type DeviceInfo, type OrgInfo, type CommandResult } from '../api';
import DeviceCard from '../components/DeviceCard';

export default function DashboardPage() {
  const [searchParams] = useSearchParams();
  const orgId = searchParams.get('org_id');

  const [devices, setDevices] = useState<DeviceInfo[]>([]);
  const [orgName, setOrgName] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Command execution state
  const [showCommandPanel, setShowCommandPanel] = useState(false);
  const [commandInput, setCommandInput] = useState('');
  const [commandLoading, setCommandLoading] = useState(false);
  const [commandError, setCommandError] = useState('');
  const [commandResults, setCommandResults] = useState<CommandResult[] | null>(null);

  useEffect(() => {
    setLoading(true);
    setError('');

    const fetchData = async () => {
      try {
        const deviceList = await api.listDevices(orgId || undefined);
        setDevices(deviceList);

        // Get org name if filtering by org
        if (orgId) {
          const orgs = await api.listOrgs();
          const org = orgs.find((o: OrgInfo) => o.id === orgId);
          setOrgName(org?.name || '');
        } else {
          setOrgName('');
        }
      } catch (err: any) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [orgId]);

  // Reset command panel when org changes
  useEffect(() => {
    setShowCommandPanel(false);
    setCommandInput('');
    setCommandResults(null);
    setCommandError('');
  }, [orgId]);

  const runCommand = async (dryRun: boolean) => {
    if (!commandInput.trim() || !orgId) return;

    setCommandLoading(true);
    setCommandError('');
    setCommandResults(null);

    try {
      const response = await api.runCommand(commandInput.trim(), orgId, dryRun);
      setCommandResults(response.results);
    } catch (err: any) {
      setCommandError(err.message);
    } finally {
      setCommandLoading(false);
    }
  };

  if (loading) return <div className="loading">Loading devices...</div>;
  if (error) return <div className="error-msg">{error}</div>;

  const addDeviceLink = orgId ? `/dashboard/add?org_id=${orgId}` : '/dashboard/add';
  const pageTitle = orgName ? orgName : 'Your Devices';

  return (
    <div className="dashboard-page">
      <div className="page-header">
        <h1>{pageTitle}</h1>
        <div className="page-header-actions">
          {orgId && (
            <button
              className="btn btn-secondary"
              onClick={() => setShowCommandPanel(!showCommandPanel)}
            >
              {showCommandPanel ? 'Close' : 'Run Command'}
            </button>
          )}
          <Link to={addDeviceLink} className="btn">Add Device</Link>
        </div>
      </div>

      {showCommandPanel && orgId && (
        <div className="command-panel">
          <div className="command-input-row">
            <input
              type="text"
              className="command-input"
              placeholder="Enter shell command (e.g. apt-get update)"
              value={commandInput}
              onChange={e => setCommandInput(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' && !commandLoading) runCommand(false);
              }}
              disabled={commandLoading}
            />
            <button
              className="btn btn-secondary"
              onClick={() => runCommand(true)}
              disabled={commandLoading || !commandInput.trim()}
            >
              Dry Run
            </button>
            <button
              className="btn"
              onClick={() => runCommand(false)}
              disabled={commandLoading || !commandInput.trim()}
            >
              Execute
            </button>
          </div>

          {commandLoading && (
            <div className="command-loading">Running command on devices...</div>
          )}

          {commandError && (
            <div className="error-msg">{commandError}</div>
          )}

          {commandResults && (
            <div className="command-results">
              <h3>Results ({commandResults.length} device{commandResults.length !== 1 ? 's' : ''})</h3>
              {commandResults.map(result => (
                <div key={result.device_id} className="command-result-card">
                  <div className="command-result-header">
                    <span className="command-result-subdomain">{result.subdomain}</span>
                    {result.error ? (
                      <span className="badge badge-error">error</span>
                    ) : (
                      <span className={`badge ${result.exit_code === 0 ? 'badge-online' : 'badge-error'}`}>
                        exit {result.exit_code}
                      </span>
                    )}
                  </div>
                  {result.error && (
                    <div className="command-result-error">{result.error}</div>
                  )}
                  {result.output && (
                    <pre className="command-result-output">{result.output}</pre>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {devices.length === 0 ? (
        <div className="empty-state">
          <p>No devices {orgName ? `in ${orgName}` : 'yet'}.</p>
          <p>
            <Link to={addDeviceLink}>Create a new device</Link> or claim an existing one with its token.
          </p>
        </div>
      ) : (
        <div className="device-grid">
          {devices.map(d => <DeviceCard key={d.id} device={d} />)}
        </div>
      )}
    </div>
  );
}
