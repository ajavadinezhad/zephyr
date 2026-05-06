const apiBase = '';

window.onerror = function(msg, url, lineNo, columnNo, error) {
  console.error('JS Error:', msg, 'at', url, lineNo, columnNo);
  return false;
};

document.addEventListener('DOMContentLoaded', () => {
  console.log('DOM loaded, setting up event listeners...');

  try {
    // Tab switching
    const tabs = document.querySelectorAll('.tab');
    console.log('Found tabs:', tabs.length);
    if (tabs.length === 0) {
      console.error('No tabs found! Check HTML class="tab"');
    }
    tabs.forEach((tab, idx) => {
      console.log('Setting up tab', idx, ':', tab.textContent, 'data-tab:', tab.dataset.tab);
      tab.addEventListener('click', () => {
        console.log('Tab clicked:', tab.dataset.tab);
        document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        tab.classList.add('active');
        const tabContent = document.getElementById('tab-' + tab.dataset.tab);
        if (tabContent) {
          tabContent.classList.add('active');
          console.log('Activated tab content:', 'tab-' + tab.dataset.tab);
        } else {
          console.error('Tab content not found:', 'tab-' + tab.dataset.tab);
        }
      });
    });

    // Circle button
    const circleBtn = document.getElementById('vpnCircle');
    if (circleBtn) {
      console.log('Setting up circle button');
      circleBtn.addEventListener('click', async () => {
        console.log('Circle button clicked');
        const isRunning = circleBtn.classList.contains('running');
        if (isRunning) {
          await stopProxy();
        } else {
          const configOk = await checkConfig();
          if (!configOk) {
            alert('Please configure Apps Script URL and Auth Key before starting the proxy.');
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            document.querySelector('[data-tab="config"]').classList.add('active');
            document.getElementById('tab-config').classList.add('active');
            return;
          }
          await startProxy();
        }
      });
    } else {
      console.error('Circle button not found! Check id="vpnCircle"');
    }

    // Load config and status
    loadConfig();
    loadStatus();
    setInterval(loadStatus, 5000);
    const listenEl = document.getElementById('listen');
    if (listenEl) {
      listenEl.addEventListener('input', updateListenDisplay);
    }

    // Start polling logs
    pollLogs();
    setInterval(pollLogs, 2000);

    // Info button toggle
    const infoBtn = document.getElementById('scriptInfoBtn');
    const popup = document.getElementById('urlListPopup');
    const closePopup = document.getElementById('closePopup');
    if (infoBtn && popup) {
      infoBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        popup.style.display = popup.style.display === 'none' ? 'block' : 'none';
      });
      document.addEventListener('click', () => { popup.style.display = 'none'; });
      popup.addEventListener('click', (e) => e.stopPropagation());
    }
    if (closePopup && popup) {
      closePopup.addEventListener('click', () => { popup.style.display = 'none'; });
    }

    console.log('All event listeners set up');
  } catch (e) {
    console.error('Error setting up event listeners:', e);
    alert('Setup error: ' + e.message);
  }
});

async function pollLogs() {
  try {
    const resp = await fetch(apiBase + '/api/logs');
    const data = await resp.json();
    const logOutput = document.getElementById('logOutput');
    if (logOutput && data.logs) {
      logOutput.textContent = data.logs.join('');
      logOutput.scrollTop = logOutput.scrollHeight;
    }
  } catch (err) {
    console.error('Failed to fetch logs:', err);
  }
}

// Load config on page load
async function loadConfig() {
  try {
    const resp = await fetch(apiBase + '/api/config');
    const data = await resp.json();
    if (data.config) {
      for (const [key, value] of Object.entries(data.config)) {
        const input = document.getElementById(key);
        if (input) input.value = value;
      }
    }
    updateListenDisplay();
  } catch (err) {
    console.error('Failed to load config:', err);
  }
}

// Load status and update UI
async function loadStatus() {
  try {
    const resp = await fetch(apiBase + '/api/status');
    const data = await resp.json();
    
    const circleBtn = document.getElementById('vpnCircle');
    const circleText = document.getElementById('circleText');
    const statusText = document.getElementById('statusText');
    const listenText = document.getElementById('listenText');
    
    if (data.proxy_running) {
      circleBtn.classList.add('running');
      circleText.textContent = 'ON';
      statusText.textContent = 'Proxy is running';
    } else {
      circleBtn.classList.remove('running');
      circleText.textContent = 'OFF';
      statusText.textContent = 'Proxy is stopped';
    }
    
    const listen = document.getElementById('listen').value || '127.0.0.1:8085';
    listenText.textContent = listen;
    
    // Render grouped script card (Android Parity)
    const scriptsSection = document.getElementById('scriptsSection');
    const scriptLabel = document.getElementById('scriptLabel');
    const scriptSubtext = document.getElementById('scriptSubtext');
    const scriptDot = document.getElementById('scriptDot');
    const urlListContent = document.getElementById('urlListContent');

    if (data.scripts && data.scripts.length > 0) {
      scriptsSection.style.display = 'block';
      scriptDot.className = 'script-dot' + (data.proxy_running ? ' active' : '');
      
      // Use the label of the first script as the primary label
      scriptLabel.textContent = data.scripts[0].label;
      
      // Show count if there are more
      if (data.scripts.length > 1) {
        scriptSubtext.textContent = `${data.scripts.length} Apps Script URLs`;
      } else {
        scriptSubtext.textContent = '1 Apps Script URL';
      }

      // Populate popup content
      urlListContent.innerHTML = '';
      data.scripts.forEach((script, idx) => {
        const entry = document.createElement('div');
        entry.className = 'url-entry';
        const shortUrl = script.id.split('/macros/s/')[1] ? '…' + script.id.split('/macros/s/')[1].substring(0, 15) + '…' : script.id;
        entry.textContent = `${idx + 1}. ${shortUrl}`;
        entry.title = script.id;
        urlListContent.appendChild(entry);
      });
    } else {
      scriptsSection.style.display = 'none';
    }
    
    updateConfigCheck();
  } catch (err) {
    console.error('Failed to load status:', err);
  }
}

function updateListenDisplay() {
  const listen = document.getElementById('listen').value || '127.0.0.1:8085';
  document.getElementById('listenText').textContent = listen;
}

// Config validation check
async function updateConfigCheck() {
  try {
    const resp = await fetch(apiBase + '/api/config');
    const data = await resp.json();
    const config = data.config || {};
    
    const checkAppScript = document.getElementById('checkAppScript');
    const iconAppScript = checkAppScript.querySelector('.check-icon');
    if (config['fronted-appscript-url']) {
      iconAppScript.textContent = '\u2713';
      iconAppScript.className = 'check-icon ok';
    } else {
      iconAppScript.textContent = '\u2717';
      iconAppScript.className = 'check-icon fail';
    }
    
    const checkAuthKey = document.getElementById('checkAuthKey');
    const iconAuthKey = checkAuthKey.querySelector('.check-icon');
    if (config['auth-key']) {
      iconAuthKey.textContent = '\u2713';
      iconAuthKey.className = 'check-icon ok';
    } else {
      iconAuthKey.textContent = '\u2717';
      iconAuthKey.className = 'check-icon fail';
    }
    
    const checkCA = document.getElementById('checkCA');
    const iconCA = checkCA.querySelector('.check-icon');
    const statusResp = await fetch(apiBase + '/api/status');
    const statusData = await statusResp.json();
    if (statusData.ca_exists) {
      iconCA.textContent = '\u2713';
      iconCA.className = 'check-icon ok';
    } else {
      iconCA.textContent = '\u2717';
      iconCA.className = 'check-icon fail';
    }
  } catch (err) {
    console.error('Failed to update config check:', err);
  }
}

// Save config
document.getElementById('configForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const config = {};
  const fields = ['fronted-appscript-url', 'auth-key', 'listen', 'front-domain', 'proxy', 'timeout'];
  fields.forEach(field => {
    const input = document.getElementById(field);
    if (input && input.value) config[field] = input.value;
  });
  
  try {
    const resp = await fetch(apiBase + '/api/config', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({config})
    });
    const data = await resp.json();
    if (data.success) {
      alert('Config saved successfully!');
      updateListenDisplay();
      updateConfigCheck();
    } else {
      alert('Failed to save config: ' + (data.error || 'Unknown error'));
    }
  } catch (err) {
    alert('Failed to save config: ' + err.message);
  }
});

// Export config for Android
document.getElementById('exportConfigBtn').addEventListener('click', async () => {
  try {
    const resp = await fetch(apiBase + '/api/export-config');
    const data = await resp.json();
    if (data.config) {
      prompt('Copy this config to import into Android app:', data.config);
    }
  } catch (err) {
    alert('Failed to export config: ' + err.message);
  }
});

// Generate CA
document.getElementById('initCaBtn').addEventListener('click', async () => {
  const status = document.getElementById('caActionStatus');
  status.textContent = 'Generating...';
  status.className = '';
  try {
    const resp = await fetch(apiBase + '/api/init-ca', {method: 'POST'});
    const data = await resp.json();
    if (data.success) {
      status.textContent = 'CA generated successfully!';
      status.className = 'success';
      loadStatus();
    } else {
      status.textContent = 'Failed: ' + (data.error || 'Unknown error');
      status.className = 'error';
    }
  } catch (err) {
    status.textContent = 'Failed: ' + err.message;
    status.className = 'error';
  }
});

async function checkConfig() {
  try {
    const resp = await fetch(apiBase + '/api/config');
    const data = await resp.json();
    const config = data.config || {};
    return !!(config['fronted-appscript-url'] && config['auth-key']);
  } catch (err) {
    return false;
  }
}

async function startProxy() {
  const statusText = document.getElementById('statusText');
  statusText.textContent = 'Starting...';
  try {
    const resp = await fetch(apiBase + '/api/proxy/start', {method: 'POST'});
    const data = await resp.json();
    if (data.success) {
      loadStatus();
    } else {
      alert('Failed to start proxy: ' + (data.error || 'Unknown error'));
      loadStatus();
    }
  } catch (err) {
    alert('Failed to start proxy: ' + err.message);
    loadStatus();
  }
}

async function stopProxy() {
  const statusText = document.getElementById('statusText');
  statusText.textContent = 'Stopping...';
  try {
    const resp = await fetch(apiBase + '/api/proxy/stop', {method: 'POST'});
    const data = await resp.json();
    if (data.success) {
      loadStatus();
    } else {
      alert('Failed to stop proxy: ' + (data.error || 'Unknown error'));
      loadStatus();
    }
  } catch (err) {
    alert('Failed to stop proxy: ' + err.message);
    loadStatus();
  }
}

// Test relay
document.getElementById('testRelayBtn').addEventListener('click', async () => {
  const status = document.getElementById('testStatus');
  const output = document.getElementById('testOutput');
  status.textContent = 'Testing...';
  status.className = '';
  output.style.display = 'none';
  const url = document.getElementById('testUrl').value;
  try {
    const resp = await fetch(apiBase + '/api/test-relay', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({url})
    });
    const data = await resp.json();
    output.style.display = 'block';
    output.textContent = JSON.stringify(data, null, 2);
    if (data.success) {
      status.textContent = 'Test passed!';
      status.className = 'success';
    } else {
      status.textContent = 'Test failed';
      status.className = 'error';
    }
  } catch (err) {
    status.textContent = 'Failed: ' + err.message;
    status.className = 'error';
  }
});

// Run probes
document.getElementById('runProbesBtn').addEventListener('click', async () => {
  const status = document.getElementById('probeStatus');
  const output = document.getElementById('probeOutput');
  status.textContent = 'Running probes...';
  status.className = '';
  output.style.display = 'none';
  const category = document.getElementById('category').value;
  try {
    const resp = await fetch(apiBase + '/api/run-probes', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({category})
    });
    const data = await resp.json();
    output.style.display = 'block';
    output.textContent = JSON.stringify(data, null, 2);
    status.textContent = 'Probes completed!';
    status.className = 'success';
  } catch (err) {
    status.textContent = 'Failed: ' + err.message;
    status.className = 'error';
  }
});

// Quit button
document.getElementById('quitBtn').addEventListener('click', async () => {
  if (!confirm('Are you sure you want to quit Zyrln?')) {
    return;
  }
  // Attempt to tell the backend to quit, but proceed to close the tab regardless
  try {
    await fetch(apiBase + '/api/quit', {method: 'POST'});
  } catch (err) {
    // Backend may already be stopping; still attempt to close window
    console.log('Quit request may have failed or completed:', err);
  }

  // After 500ms, try to close the browser tab. If the browser blocks window.close(), fall back to a blank tab.
  setTimeout(() => {
    try {
      window.close();
    } catch (e) {
      // Ignore: some browsers console error when closing programmatically
    }
    // Fallback: navigate to a blank page to effectively leave the tab
    try {
      if (!window.closed) {
        window.location.href = 'about:blank';
      }
    } catch (e) {
      // ignore
    }
  }, 500);
});
