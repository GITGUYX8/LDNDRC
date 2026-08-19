import './style.css';

const DEFAULT_WS_URL = 'ws://localhost:9002';

const wsUrlInput = document.querySelector('#wsUrl');
const autoConnectInput = document.querySelector('#autoConnect');
const connectButton = document.querySelector('#connectButton');
const disconnectButton = document.querySelector('#disconnectButton');
const clearLogButton = document.querySelector('#clearLogButton');
const statusBadge = document.querySelector('#statusBadge');
const sceneElement = document.querySelector('#gzScene');
const debugLog = document.querySelector('#debugLog');

let sceneManager = null;
let statusSubscription = null;
let renderWatchdog = null;
let retryTimer = null;

function timestamp() {
  return new Date().toLocaleTimeString();
}

function log(message, detail) {
  const suffix = detail === undefined ? '' : ` ${formatDetail(detail)}`;
  const line = `[${timestamp()}] ${message}${suffix}`;
  debugLog.textContent += `${line}\n`;
  debugLog.scrollTop = debugLog.scrollHeight;
  console.log(line, detail ?? '');
}

function warn(message, detail) {
  const suffix = detail === undefined ? '' : ` ${formatDetail(detail)}`;
  const line = `[${timestamp()}] WARN: ${message}${suffix}`;
  debugLog.textContent += `${line}\n`;
  debugLog.scrollTop = debugLog.scrollHeight;
  console.warn(line, detail ?? '');
}

function fail(message, error) {
  setStatus('error', 'Error');
  const normalized = normalizeError(error);
  const line = `[${timestamp()}] ERROR: ${message} ${normalized}`;
  debugLog.textContent += `${line}\n`;
  debugLog.scrollTop = debugLog.scrollHeight;
  console.error(message, error);
}

function formatDetail(detail) {
  if (typeof detail === 'string') {
    return detail;
  }

  try {
    return JSON.stringify(detail);
  } catch {
    return String(detail);
  }
}

function normalizeError(error) {
  if (!error) {
    return '';
  }

  if (error instanceof Error) {
    return `${error.name}: ${error.message}`;
  }

  return formatDetail(error);
}

function setStatus(state, label) {
  statusBadge.dataset.state = state;
  statusBadge.textContent = label;
}

function resetSceneElement() {
  sceneElement.innerHTML = '<div class="empty-state">Connecting to Gazebo...</div>';
}

function validateWebSocketUrl(rawUrl) {
  const url = new URL(rawUrl);

  if (url.protocol !== 'ws:' && url.protocol !== 'wss:') {
    throw new Error('WebSocket URL must start with ws:// or wss://');
  }

  if (window.location.protocol === 'https:' && url.protocol === 'ws:') {
    warn('The page is loaded over HTTPS but the Gazebo URL uses ws://. Browsers usually block mixed-content websockets. Use wss:// through your gateway.');
  }

  return url.toString();
}

async function loadGzweb() {
  log('Loading gzweb package...');
  const module = await import('gzweb');
  log('Loaded gzweb module exports:', Object.keys(module));

  const SceneManager =
    module.SceneManager ||
    module.default?.SceneManager ||
    module.default;

  if (typeof SceneManager !== 'function') {
    throw new Error('Could not find a SceneManager export in the gzweb package.');
  }

  return SceneManager;
}

function attachStatusObserver(manager) {
  if (typeof manager.getConnectionStatus === 'function') {
    log(`Initial gzweb status: ${manager.getConnectionStatus()}`);
  }

  if (typeof manager.getConnectionStatusAsObservable !== 'function') {
    warn('gzweb SceneManager does not expose getConnectionStatusAsObservable(); status polling will be limited.');
    return;
  }

  const observable = manager.getConnectionStatusAsObservable();
  if (!observable || typeof observable.subscribe !== 'function') {
    warn('gzweb connection status observable does not look subscribable.');
    return;
  }

  statusSubscription = observable.subscribe((status) => {
    log(`gzweb status: ${status}`);

    if (status === true) {
      setStatus('connected', 'Ready');
    } else if (String(status).toLowerCase().includes('connected')) {
      setStatus('connected', 'Connected');
    }
  });
}

function probeWebSocket(url, timeoutMs = 2500) {
  return new Promise((resolve, reject) => {
    let socket;
    let settled = false;

    const timeout = window.setTimeout(() => {
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.close(1000, 'probe timeout');
      }

      if (!settled) {
        settled = true;
        reject(new Error(`WebSocket probe timed out after ${timeoutMs}ms`));
      }
    }, timeoutMs);

    try {
      socket = new WebSocket(url);
    } catch (error) {
      window.clearTimeout(timeout);
      reject(error);
      return;
    }

    socket.addEventListener('open', () => {
      log('Raw WebSocket probe opened. The endpoint accepted a browser websocket upgrade.');
      socket.close(1000, 'probe complete');

      if (!settled) {
        settled = true;
        window.clearTimeout(timeout);
        resolve();
      }
    });

    socket.addEventListener('message', (event) => {
      const length = typeof event.data === 'string' ? event.data.length : event.data?.size ?? event.data?.byteLength ?? 'unknown';
      log('Raw WebSocket probe received a message.', { type: typeof event.data, length });
    });

    socket.addEventListener('close', (event) => {
      log('Raw WebSocket probe closed.', {
        code: event.code,
        reason: event.reason || '(no reason)',
        wasClean: event.wasClean
      });
    });

    socket.addEventListener('error', (event) => {
      if (!settled) {
        settled = true;
        window.clearTimeout(timeout);
        reject(event);
      }
    });
  });
}

function scheduleRetry() {
  if (!autoConnectInput.checked || retryTimer) {
    return;
  }

  setStatus('waiting', 'Waiting');
  log('Gazebo websocket is not available yet. Retrying in 3 seconds...');
  retryTimer = window.setTimeout(() => {
    retryTimer = null;
    connect().catch((error) => {
      fail('Unexpected auto-connect failure.', error);
      scheduleRetry();
    });
  }, 3000);
}

async function connect() {
  disconnect();

  const url = validateWebSocketUrl(wsUrlInput.value.trim() || DEFAULT_WS_URL);
  wsUrlInput.value = url;

  setStatus('connecting', 'Connecting');
  connectButton.disabled = true;
  disconnectButton.disabled = false;
  resetSceneElement();
  log(`Connecting to ${url}`);

  try {
    await probeWebSocket(url);

    const SceneManager = await loadGzweb();
    sceneElement.innerHTML = '';

    sceneManager = new SceneManager({
      elementId: 'gzScene',
      websocketUrl: url,
      enableLights: true
    });

    window.__gazeboSceneManager = sceneManager;
    log('Created gzweb SceneManager. Exposed it as window.__gazeboSceneManager for browser console inspection.');
    attachStatusObserver(sceneManager);

    renderWatchdog = window.setTimeout(() => {
      const hasCanvas = Boolean(sceneElement.querySelector('canvas'));

      if (!hasCanvas) {
        warn('No canvas was created after 10 seconds. The websocket may be reachable, but gzweb could not initialize rendering or scene data.');
      } else {
        log('Canvas exists. If the scene is still blank, inspect browser console WebGL and gzweb errors.');
      }
    }, 10000);
  } catch (error) {
    fail('Failed to connect or start gzweb rendering.', error);
    connectButton.disabled = false;
    disconnectButton.disabled = true;
    scheduleRetry();
  }
}

function disconnect() {
  if (retryTimer) {
    window.clearTimeout(retryTimer);
    retryTimer = null;
  }

  if (renderWatchdog) {
    window.clearTimeout(renderWatchdog);
    renderWatchdog = null;
  }

  if (statusSubscription && typeof statusSubscription.unsubscribe === 'function') {
    statusSubscription.unsubscribe();
  }
  statusSubscription = null;

  if (sceneManager) {
    if (typeof sceneManager.disconnect === 'function') {
      sceneManager.disconnect();
    } else if (typeof sceneManager.destroy === 'function') {
      sceneManager.destroy();
    } else if (typeof sceneManager.dispose === 'function') {
      sceneManager.dispose();
    }
  }
  sceneManager = null;

  sceneElement.innerHTML = '<div class="empty-state">Disconnected. Enter the websocket URL and connect again.</div>';
  setStatus('idle', 'Idle');
  connectButton.disabled = false;
  disconnectButton.disabled = true;
}

connectButton.addEventListener('click', () => {
  connect().catch((error) => {
    fail('Unexpected connect failure.', error);
    connectButton.disabled = false;
    disconnectButton.disabled = true;
  });
});

disconnectButton.addEventListener('click', disconnect);

clearLogButton.addEventListener('click', () => {
  debugLog.textContent = '';
});

window.addEventListener('resize', () => {
  if (sceneManager && typeof sceneManager.resize === 'function') {
    sceneManager.resize();
  }
});

window.addEventListener('error', (event) => {
  fail('Unhandled browser error.', event.error || event.message);
});

window.addEventListener('unhandledrejection', (event) => {
  fail('Unhandled promise rejection.', event.reason);
});

setStatus('idle', 'Idle');
log('Viewer ready. Default URL is ws://localhost:9002.');

if (autoConnectInput.checked) {
  log('Auto-connect is enabled. Waiting for Gazebo websocket to appear...');
  scheduleRetry();
}
