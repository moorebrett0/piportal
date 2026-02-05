import { useEffect, useRef, useState } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

interface TerminalProps {
  deviceId: string;
  onDisconnect: () => void;
}

export default function Terminal({ deviceId, onDisconnect }: TerminalProps) {
  const termRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');

  useEffect(() => {
    if (!termRef.current) return;

    const xterm = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: "'SF Mono', Menlo, Monaco, 'Courier New', monospace",
      theme: {
        background: '#0f1535',
        foreground: '#e2e8f0',
        cursor: '#0075ff',
        selectionBackground: 'rgba(0, 117, 255, 0.3)',
        black: '#0f1535',
        red: '#e31a1a',
        green: '#01b574',
        yellow: '#ffb547',
        blue: '#0075ff',
        magenta: '#b33dc6',
        cyan: '#21d4fd',
        white: '#e2e8f0',
        brightBlack: '#475569',
        brightRed: '#ff6b6b',
        brightGreen: '#34d399',
        brightYellow: '#fcd34d',
        brightBlue: '#60a5fa',
        brightMagenta: '#c084fc',
        brightCyan: '#67e8f9',
        brightWhite: '#ffffff',
      },
    });

    const fitAddon = new FitAddon();
    xterm.loadAddon(fitAddon);
    xterm.open(termRef.current);
    fitAddon.fit();

    xtermRef.current = xterm;
    fitRef.current = fitAddon;

    xterm.writeln('\x1b[1;34mConnecting to device...\x1b[0m');

    // Build WebSocket URL
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${proto}//${window.location.host}/api/v1/devices/${deviceId}/terminal`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      // Send initial size
      const dims = fitAddon.proposeDimensions();
      ws.send(JSON.stringify({
        rows: dims?.rows ?? 24,
        cols: dims?.cols ?? 80,
      }));
      setStatus('connected');
      xterm.writeln('\x1b[1;32mConnected.\x1b[0m\r\n');
      xterm.focus();
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'terminal_data' && msg.data_base64) {
          const bytes = Uint8Array.from(atob(msg.data_base64), c => c.charCodeAt(0));
          xterm.write(bytes);
        } else if (msg.type === 'terminal_close') {
          xterm.writeln('\r\n\x1b[1;31mSession closed.\x1b[0m');
          setStatus('disconnected');
        }
      } catch {
        // ignore parse errors
      }
    };

    ws.onclose = () => {
      if (status !== 'disconnected') {
        xterm.writeln('\r\n\x1b[1;31mDisconnected.\x1b[0m');
        setStatus('disconnected');
      }
    };

    ws.onerror = () => {
      xterm.writeln('\r\n\x1b[1;31mConnection error.\x1b[0m');
      setStatus('disconnected');
    };

    // Send terminal input to server
    const inputDisposable = xterm.onData((data: string) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ data }));
      }
    });

    // Handle resize
    const handleResize = () => {
      fitAddon.fit();
      const dims = fitAddon.proposeDimensions();
      if (dims && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'resize',
          rows: dims.rows,
          cols: dims.cols,
        }));
      }
    };

    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(termRef.current);

    return () => {
      inputDisposable.dispose();
      resizeObserver.disconnect();
      ws.close();
      xterm.dispose();
      wsRef.current = null;
      xtermRef.current = null;
      fitRef.current = null;
    };
  }, [deviceId]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="terminal-container">
      <div className="terminal-header">
        <div className="terminal-title">
          <span className={`terminal-dot terminal-dot-${status}`} />
          Terminal
          <span className="terminal-status">
            {status === 'connecting' && 'Connecting...'}
            {status === 'connected' && 'Connected'}
            {status === 'disconnected' && 'Disconnected'}
          </span>
        </div>
        <button className="btn-link" onClick={onDisconnect}>Close</button>
      </div>
      <div ref={termRef} className="terminal-viewport" />
    </div>
  );
}
