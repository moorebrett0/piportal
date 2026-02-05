import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';

export default function AddDevicePage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState<'create' | 'claim'>('create');

  // Create state
  const [subdomain, setSubdomain] = useState('');
  const [createError, setCreateError] = useState('');
  const [creating, setCreating] = useState(false);
  const [createdToken, setCreatedToken] = useState('');

  // Claim state
  const [token, setToken] = useState('');
  const [claimError, setClaimError] = useState('');
  const [claiming, setClaiming] = useState(false);

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setCreateError('');
    setCreating(true);
    try {
      const res = await api.createDevice(subdomain);
      setCreatedToken(res.token);
    } catch (err: any) {
      setCreateError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleClaim = async (e: FormEvent) => {
    e.preventDefault();
    setClaimError('');
    setClaiming(true);
    try {
      const res = await api.claimDevice(token);
      navigate(`/dashboard/devices/${res.id}`);
    } catch (err: any) {
      setClaimError(err.message);
    } finally {
      setClaiming(false);
    }
  };

  if (createdToken) {
    return (
      <div className="add-page">
        <h1>Device Created</h1>
        <div className="success-box">
          <p>Your device <strong>{subdomain}</strong> has been created.</p>
          <p>Save this token — you'll need it to connect your Pi:</p>
          <pre className="code-block">{createdToken}</pre>
          <p>Then run on your Pi:</p>
          <pre className="code-block">
{`curl -fsSL https://piportal.dev/install.sh | bash
piportal setup`}
          </pre>
          <p>When prompted for a token, paste the token above.</p>
          <button onClick={() => navigate('/dashboard')} className="btn">
            Go to Dashboard
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="add-page">
      <h1>Add Device</h1>

      <div className="tabs">
        <button
          className={`tab ${tab === 'create' ? 'tab-active' : ''}`}
          onClick={() => setTab('create')}
        >
          Create New
        </button>
        <button
          className={`tab ${tab === 'claim' ? 'tab-active' : ''}`}
          onClick={() => setTab('claim')}
        >
          Claim Existing
        </button>
      </div>

      {tab === 'create' ? (
        <form onSubmit={handleCreate} className="auth-form">
          {createError && <div className="error-msg">{createError}</div>}
          <label>
            Subdomain
            <div className="subdomain-input">
              <input
                type="text"
                value={subdomain}
                onChange={e => setSubdomain(e.target.value.toLowerCase())}
                required
                pattern="[a-z0-9][a-z0-9-]{1,28}[a-z0-9]"
                placeholder="my-pi"
                autoFocus
              />
              <span className="subdomain-suffix">.piportal.dev</span>
            </div>
          </label>
          <button type="submit" className="btn" disabled={creating}>
            {creating ? 'Creating...' : 'Create Device'}
          </button>
        </form>
      ) : (
        <form onSubmit={handleClaim} className="auth-form">
          {claimError && <div className="error-msg">{claimError}</div>}
          <label>
            Device Token
            <input
              type="text"
              value={token}
              onChange={e => setToken(e.target.value)}
              required
              placeholder="pp_..."
              autoFocus
            />
          </label>
          <p className="form-hint">
            Enter the token from <code>piportal setup</code> to link an existing device to your account.
          </p>
          <button type="submit" className="btn" disabled={claiming}>
            {claiming ? 'Claiming...' : 'Claim Device'}
          </button>
        </form>
      )}
    </div>
  );
}
