let endpointsData = {};
let currentFilter = 'all';
let searchQuery = '';
let filterUnhealthy = false;
let filterExpiringCerts = false;
let appConfig = { ssl_expiry_warning_days: 30, has_passkey: false };

// Load config on startup
async function loadConfig() {
    try {
        const resp = await fetch('/api/config');
        if (resp.ok) {
            appConfig = await resp.json();
        }
    } catch (e) {
        console.error('Failed to load config:', e);
    }
}

function formatDuration(ms) {
    if (ms < 1000) return ms.toFixed(0) + 'ms';
    return (ms / 1000).toFixed(2) + 's';
}

function formatTime(timestamp) {
    return new Date(timestamp).toLocaleTimeString();
}

function formatInterval(ns) {
    if (!ns) return '30s';
    const seconds = ns / 1000000000;
    if (seconds >= 60) return Math.round(seconds / 60) + 'm';
    return Math.round(seconds) + 's';
}

function formatSSLDate(dateString) {
    if (!dateString) return 'N/A';
    try {
        const date = new Date(dateString);
        if (isNaN(date.getTime())) return 'N/A';
        const day = String(date.getDate()).padStart(2, '0');
        const month = String(date.getMonth() + 1).padStart(2, '0');
        const year = date.getFullYear();
        return `${year}-${month}-${day}`;
    } catch (e) {
        return 'N/A';
    }
}

function handleSearchInput(value) {
    searchQuery = value.toLowerCase().trim();
    renderEndpoints();
}

function toggleFilter(filterType) {
    if (filterType === 'unhealthy') {
        filterUnhealthy = !filterUnhealthy;
        const toggle = document.getElementById('toggleUnhealthy');
        if (filterUnhealthy) {
            toggle.classList.add('active');
        } else {
            toggle.classList.remove('active');
        }
    } else if (filterType === 'expiringCerts') {
        filterExpiringCerts = !filterExpiringCerts;
        const toggle = document.getElementById('toggleExpiringCerts');
        if (filterExpiringCerts) {
            toggle.classList.add('active');
        } else {
            toggle.classList.remove('active');
        }
    }
    renderEndpoints();
}

function setFilter(filter) {
    currentFilter = filter;

    // Update active state on stat cards
    document.querySelectorAll('.stat-card').forEach(card => {
        if (card.dataset.filter === filter) {
            card.classList.add('active');
        } else {
            card.classList.remove('active');
        }
    });

    // Re-render endpoints with current filter
    renderEndpoints();
}

function shouldShowEndpoint(endpoint) {
    const isEnabled = endpoint.enabled !== false;

    // 1. Apply stat card filter
    let passesStatFilter = true;
    switch (currentFilter) {
        case 'all':
            passesStatFilter = true;
            break;
        case 'healthy':
            passesStatFilter = isEnabled && endpoint.status === 'healthy';
            break;
        case 'unhealthy':
            passesStatFilter = isEnabled && endpoint.status === 'unhealthy';
            break;
        case 'disabled':
            passesStatFilter = !isEnabled;
            break;
        case 'expiringCerts':
            passesStatFilter = endpoint.ssl_expiring_soon === true;
            break;
        default:
            passesStatFilter = true;
    }

    if (!passesStatFilter) return false;

    // 2. Apply search query
    if (searchQuery) {
        const name = (endpoint.name || '').toLowerCase();
        const url = (endpoint.url || '').toLowerCase();
        const matchesSearch = name.includes(searchQuery) || url.includes(searchQuery);
        if (!matchesSearch) return false;
    }

    // 3. Apply toggle filters (OR logic)
    if (filterUnhealthy || filterExpiringCerts) {
        const matchesUnhealthy = filterUnhealthy && endpoint.status === 'unhealthy';
        const matchesExpiringCerts = filterExpiringCerts && endpoint.ssl_expiring_soon;

        // If either toggle is on, endpoint must match at least one condition
        if (!matchesUnhealthy && !matchesExpiringCerts) return false;
    }

    return true;
}

async function loadHistoryChart(endpointId) {
    try {
        const resp = await fetch('/api/history?id=' + endpointId);
        if (!resp.ok) return;
        const data = await resp.json();
        const chart = document.getElementById('chart-' + endpointId);
        if (!chart) return;

        chart.innerHTML = '';
        const records = (data.records || []).slice(0, 50).reverse();

        if (records.length === 0) {
            chart.innerHTML = '<span style="color:#9ca3af;font-size:0.7em;margin:auto;">No history</span>';
            return;
        }

        records.slice(0, 20).forEach(record => {
            const bar = document.createElement('div');
            bar.className = 'bar';
            if (record.status === 'healthy') {
                bar.classList.add('success');
            } else if (record.status === 'unhealthy') {
                bar.classList.add('failure');
            } else {
                bar.classList.add('unknown');
            }
            const respTime = record.response_time ? formatDuration(record.response_time / 1000000) : '-';
            bar.title = record.status + ' | ' + respTime + ' | ' + new Date(record.timestamp).toLocaleString();
            chart.appendChild(bar);
        });

        // Update average response time
        const avgEl = document.getElementById('avg-' + endpointId);
        if (avgEl && data.avg_response_time_ms) {
            avgEl.textContent = formatDuration(data.avg_response_time_ms);
        }
    } catch (err) {
        console.error('Error loading history:', err);
    }
}

function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = 'toast ' + type;
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
}

function openAddModal() {
    document.getElementById('addModal').classList.add('active');
    // Reset health fields to disabled state
    toggleHealthFields(false);
    document.getElementById('ep-monitor-health').checked = false;
}

function closeAddModal() {
    document.getElementById('addModal').classList.remove('active');
    document.getElementById('addForm').reset();
    toggleHealthFields(false);
}

function toggleHealthFields(enabled) {
    const healthFields = document.getElementById('health-fields');
    const inputs = healthFields.querySelectorAll('input, select');

    if (enabled) {
        healthFields.classList.remove('disabled');
        inputs.forEach(input => input.disabled = false);
    } else {
        healthFields.classList.add('disabled');
        inputs.forEach(input => input.disabled = true);
    }
}

async function addEndpoint(e) {
    e.preventDefault();

    // Validate and normalize URL (reusing old validation logic)
    let url = document.getElementById('ep-url').value.trim();

    // Ensure URL has proper protocol format with ://
    if (!url.includes('://')) {
        showToast('Invalid URL: must include protocol (e.g., https://)', 'error');
        return;
    }

    const monitorHealth = document.getElementById('ep-monitor-health').checked;
    const selectedInterval = document.getElementById('ep-interval').value || '1m';

    const data = {
        name: document.getElementById('ep-name').value,
        url: url,
        monitor_health: monitorHealth,
        method: monitorHealth ? document.getElementById('ep-method').value : 'GET',
        check_interval: monitorHealth ? selectedInterval : '0s',
        timeout: monitorHealth ? document.getElementById('ep-timeout').value : '10s',
        expected_status: monitorHealth ? (parseInt(document.getElementById('ep-status').value) || 200) : 200,
        failure_threshold: monitorHealth ? (parseInt(document.getElementById('ep-failure').value) || 3) : 3,
        success_threshold: monitorHealth ? (parseInt(document.getElementById('ep-success').value) || 2) : 2
    };
    try {
        const resp = await fetch('/api/endpoints/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (resp.ok) {
            showToast('Endpoint added successfully');
            closeAddModal();
            updateDashboard();
        } else {
            const err = await resp.text();
            showToast(err, 'error');
        }
    } catch (err) {
        showToast('Failed to add endpoint', 'error');
    }
}

async function deleteEndpoint(id, name) {
    console.log('Delete endpoint called with id:', id, 'name:', name);
    if (!confirm('Delete endpoint "' + name + '"?')) return;
    try {
        console.log('Sending delete request for id:', id);
        const resp = await fetch('/api/endpoints/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        });
        console.log('Delete response status:', resp.status);
        const text = await resp.text();
        console.log('Delete response body:', text);
        if (resp.ok) {
            showToast('Endpoint deleted');
            updateDashboard();
        } else {
            showToast('Failed to delete endpoint: ' + text, 'error');
        }
    } catch (err) {
        console.error('Delete error:', err);
        showToast('Failed to delete endpoint', 'error');
    }
}

async function toggleEndpoint(id, enable) {
    const action = enable ? 'enable' : 'disable';
    try {
        const resp = await fetch('/api/endpoints/' + action, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        });
        if (resp.ok) {
            showToast('Endpoint ' + action + 'd');
            updateDashboard();
        } else {
            showToast('Failed to ' + action + ' endpoint', 'error');
        }
    } catch (err) {
        showToast('Failed to ' + action + ' endpoint', 'error');
    }
}

async function toggleAlerts(id, suppress) {
    const action = suppress ? 'suppress' : 'unsuppress';
    try {
        const resp = await fetch('/api/endpoints/' + action, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        });
        if (resp.ok) {
            showToast(suppress ? 'Alerts suppressed' : 'Alerts enabled');
            updateDashboard();
        } else {
            showToast('Failed to update alerts', 'error');
        }
    } catch (err) {
        showToast('Failed to update alerts', 'error');
    }
}

async function updateDashboard() {
    try {
        const [statusResp, endpointsResp] = await Promise.all([
            fetch('/api/status'),
            fetch('/api/endpoints')
        ]);
        const statusData = await statusResp.json();
        const endpointsDbData = await endpointsResp.json();

        // Create a map of endpoint settings from DB
        const dbEndpoints = {};
        (endpointsDbData.endpoints || []).forEach(ep => {
            dbEndpoints[ep.id] = ep;
        });

        // Combine status data with DB settings
        const allEndpoints = [];
        Object.entries(statusData.endpoints || {}).forEach(([name, endpoint]) => {
            const dbEp = Object.values(dbEndpoints).find(e => e.name === endpoint.name) || {};
            allEndpoints.push({ ...endpoint, ...dbEp, id: endpoint.id || dbEp.id || name });
        });

        // Also add any DB endpoints not in status
        Object.values(dbEndpoints).forEach(dbEp => {
            if (!allEndpoints.find(e => e.id === dbEp.id)) {
                allEndpoints.push({ ...dbEp, status: 'unknown' });
            }
        });

        // Store endpoints data globally
        endpointsData = allEndpoints;

        // Render endpoints with current filter
        renderEndpoints();
    } catch (error) {
        console.error('Error fetching status:', error);
    }
}

async function rerunSSLCheck() {
    const btn = document.getElementById('sslRecheckBtn');
    if (!btn) return;

    // Prevent double clicks
    btn.disabled = true;
    const originalText = btn.innerHTML;
    btn.innerHTML = '⏳ Running SSL Check...';

    try {
        const resp = await fetch('/api/ssl/recheck', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });

        if (!resp.ok) {
            const err = await resp.text();
            showToast('Failed to re-run SSL check: ' + err, 'error');
            return;
        }

        showToast('SSL validation triggered for all endpoints');

        // Give backend a moment, then refresh UI
        setTimeout(() => {
            updateDashboard();
        }, 2000);

    } catch (err) {
        console.error(err);
        showToast('Error triggering SSL validation', 'error');
    } finally {
        btn.disabled = false;
        btn.innerHTML = originalText;
    }
}

function renderEndpoints() {
    const allEndpoints = endpointsData;
    let healthy = 0, unhealthy = 0, disabled = 0, total = 0, expiringCerts = 0, healthMonitored = 0, sslOnly = 0;;
    let visibleCount = 0;

    const endpointsContainer = document.getElementById('endpoints');
    endpointsContainer.innerHTML = '';
    // Get priority values (lower number = higher priority)
    const getPriority = (endpoint) => {
        const isEnabled = endpoint.enabled !== false;
        const monitorHealth = endpoint.monitor_health === true;

        if (!isEnabled) return 5;

        //  Unhealthy first (health monitored only)
        if (monitorHealth && endpoint.status === 'unhealthy') return 1;

        //  SSL expiring soon (health OR ssl-only)
        if (endpoint.ssl_expiring_soon === true) return 2;

        //  Healthy (health monitored)
        if (monitorHealth && endpoint.status === 'healthy') return 3;

        //  SSL valid / SSL-only
        return 4;
    };

    // Sort endpoints by priority: unhealthy → cert expiring → healthy → certs valid
    const sortedEndpoints = [...allEndpoints].sort((a, b) => {
        const priorityA = getPriority(a);
        const priorityB = getPriority(b);

        if (priorityA !== priorityB) {
            return priorityA - priorityB;
        }

        //  Only SSL-expiring endpoints sorted by expiry date
        if (
            priorityA === 2 &&
            a.ssl_cert_expiry &&
            b.ssl_cert_expiry
        ) {
            return new Date(a.ssl_cert_expiry) - new Date(b.ssl_cert_expiry);
        }
        // Same priority: sort alphabetically by name
        return a.name.localeCompare(b.name);
    });

    sortedEndpoints.forEach(endpoint => {
        total++;
        const isEnabled = endpoint.enabled !== false;
        const isSuppressed = endpoint.alerts_suppressed === true;
        const monitorHealth = endpoint.monitor_health === true;
        const isNewEndpoint = !endpoint.last_check || endpoint.last_check === '0001-01-01T00:00:00Z';

        // Count health monitored endpoints
        if (monitorHealth) healthMonitored++;
        // ✅ SSL-only count
        if (isEnabled && !monitorHealth) {
            sslOnly++;
        }

        // Count by status - only count enabled endpoints as healthy/unhealthy
        // Disabled endpoints are counted separately
        // Unknown status endpoints are not counted as healthy or unhealthy
        if (!isEnabled) {
            disabled++;
        } else if (monitorHealth) {
            if (endpoint.status === 'healthy') healthy++;
            else if (endpoint.status === 'unhealthy') unhealthy++;
            // Note: 'unknown' status endpoints are not counted in healthy or unhealthy
        }

        // Count expiring SSL certificates
        if (endpoint.ssl_expiring_soon) expiringCerts++;

        // Apply filter
        if (!shouldShowEndpoint(endpoint)) return;

        visibleCount++;

        const row = document.createElement('div');
        row.className = 'endpoint-row ' + (monitorHealth ? endpoint.status : 'ssl-only') + (isEnabled ? '' : ' disabled');
        row.style.cursor = 'pointer';

        const sslExpiryDate = formatSSLDate(endpoint.ssl_cert_expiry);
        const sslInlineText =
            endpoint.ssl_expiring_soon && typeof endpoint.days_to_expiry === 'number'
                ? `${sslExpiryDate} · ${endpoint.days_to_expiry} days`
                : `${sslExpiryDate}`;

        const sslIcon = endpoint.ssl_expiring_soon ? '⚠️' : (endpoint.ssl_cert_expiry ? '🔒' : '');
        const sslClass = endpoint.ssl_expiring_soon ? 'ssl-danger' : (endpoint.ssl_cert_expiry ? 'ssl-valid' : 'ssl-none');

        // Badge: lock+heart for SSL+Health, lock+greyed heart for SSL only
        const monitorBadge = monitorHealth
            ? '<span class="monitor-badge" title="SSL + Health Monitoring">🔒🚦</span>'
            : '<span class="monitor-badge ssl-only-badge" title="SSL Monitoring Only">🔒<span class="greyed">🚦</span></span>';

        // History/status section - different for SSL-only vs health monitored
        let historySection;
        if (!monitorHealth) {
            if (isNewEndpoint) {
                historySection = '<div class="ssl-only-label" title="New endpoint - SSL certificate will be checked shortly">🆕 New endpoint</div>';
            } else {
                historySection = '<div class="ssl-only-label" title="Health monitoring disabled - SSL certificate only">🔒 SSL Only</div>';
            }
        } else {
            historySection = `<div class="history-mini" id="chart-${endpoint.id}"></div>`;
        }

        // Stats section - greyed out for SSL-only
        const statsClass = monitorHealth ? 'endpoint-stats' : 'endpoint-stats disabled-stats';
        const statsContent = monitorHealth ? `
            <span title="Response Time">${formatDuration(endpoint.response_time_ms || 0)}</span>
            <span class="stat-avg" title="Average Response Time" id="avg-${endpoint.id}">-</span>
            <span title="Check Interval">${formatInterval(endpoint.check_interval)}</span>
            <span class="stat-success" title="Consecutive Successes">✓${endpoint.consecutive_successes || 0}</span>
            <span class="stat-fail" title="Consecutive Failures">✗${endpoint.consecutive_failures || 0}</span>
        ` : `<span class="disabled-text" title="Health monitoring disabled">-</span>`;

        row.innerHTML = `
            <div class="endpoint-status ${monitorHealth ? endpoint.status : 'ssl-only'}" title="${monitorHealth ? 'Health: ' + endpoint.status : 'SSL Only'}"></div>
            ${monitorBadge}
            <div class="endpoint-name" title="${endpoint.name}">${endpoint.name}</div>
            <div class="ssl-expiry ${sslClass}" title="SSL Certificate expires: ${sslExpiryDate}${endpoint.days_to_expiry ? ' (' + endpoint.days_to_expiry + ' days)' : ''}">
                <span class="ssl-icon">${sslIcon}</span>
                <span>SSL: ${sslInlineText}</span>
            </div>
            <a class="endpoint-url" href="${endpoint.url}" target="_blank" rel="noopener noreferrer" title="Open ${endpoint.url} in new tab" onclick="event.stopPropagation()">${endpoint.url}</a>
            ${historySection}
            <div class="${statsClass}">
                ${statsContent}
            </div>
            <div class="endpoint-actions" data-endpoint-id="${endpoint.id}" data-endpoint-name="${endpoint.name}" 
                 data-interval="${formatInterval(endpoint.check_interval)}" data-timeout="${formatInterval(endpoint.timeout)}"
                 data-failure="${endpoint.failure_threshold || 3}" data-success="${endpoint.success_threshold || 2}"
                 data-monitor-health="${monitorHealth}">
                ${monitorHealth ? '<button class="icon-btn edit" data-action="history" title="View Health History">📊</button>' : '<button class="icon-btn edit" data-action="enable-health" title="Enable Health Monitoring">🚦</button>'}
                <button class="icon-btn edit" data-action="edit" title="Edit Endpoint Settings">✏️</button>
                <button class="icon-btn ${isEnabled ? 'toggle-on' : 'toggle-off'}" data-action="${isEnabled ? 'disable' : 'enable'}" title="${isEnabled ? 'Disable Monitoring' : 'Enable Monitoring'}">${isEnabled ? '⏸️' : '▶️'}</button>
                ${monitorHealth ? `<button class="icon-btn ${isSuppressed ? 'alert-on' : 'alert-off'}" data-action="${isSuppressed ? 'unsuppress' : 'suppress'}" title="${isSuppressed ? 'Enable Alerts' : 'Suppress Alerts'}">${isSuppressed ? '🔔' : '🔕'}</button>` : ''}
                <button class="icon-btn delete" data-action="delete" title="Delete Endpoint">🗑️</button>
            </div>
        `;

        // Make row clickable to open history (but not when clicking action buttons)
        row.addEventListener('click', function (e) {
            if (!e.target.closest('.endpoint-actions') && !e.target.closest('.endpoint-url')) {
                if (monitorHealth) {
                    openHistoryModal(endpoint.id, endpoint.name);
                }
            }
        });

        endpointsContainer.appendChild(row);

        // Load history chart for this endpoint (only if health monitoring enabled)
        if (monitorHealth) {
            loadHistoryChart(endpoint.id);
        }
    });

    // Show empty state if no endpoints match filters
    if (visibleCount === 0 && total > 0) {
        const emptyState = document.createElement('div');
        emptyState.className = 'empty-state';
        emptyState.innerHTML = `
            <div class="empty-state-icon">🔍</div>
            <h3>No endpoints match the selected filters</h3>
            <p>Try adjusting your search or filter criteria</p>
        `;
        endpointsContainer.appendChild(emptyState);
    }

    // Update stat counts
    document.getElementById('total-endpoints').innerHTML = total + ' <span class="stat-detail">| ' + sslOnly + ' 🔒 ' + healthMonitored + ' 🚦</span>';
    document.getElementById('healthy-count').textContent = healthy;
    document.getElementById('unhealthy-count').textContent = unhealthy;
    document.getElementById('expiring-certs-count').textContent = expiringCerts;
    document.getElementById('disabled-count').textContent = disabled;
    document.getElementById('last-update').textContent = new Date().toLocaleTimeString();

    // Update expiring certs card title with configured days
    const expiringCard = document.querySelector('[data-filter="expiringCerts"] h3');
    if (expiringCard) {
        expiringCard.textContent = 'Expiring in ' + appConfig.ssl_expiry_warning_days + ' days';
    }

    // Update active filter state
    document.querySelectorAll('.stat-card').forEach(card => {
        if (card.dataset.filter === currentFilter) {
            card.classList.add('active');
        } else {
            card.classList.remove('active');
        }
    });
}

// Event delegation for action buttons
document.addEventListener('click', async function (e) {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;

    const action = btn.dataset.action;
    const actionsDiv = btn.closest('.endpoint-actions');
    const id = actionsDiv ? actionsDiv.dataset.endpointId : '';
    const name = actionsDiv ? actionsDiv.dataset.endpointName : id;

    console.log('Button clicked:', action, id, name);

    if (action === 'delete') {
        if (!confirm('Delete endpoint "' + name + '"?')) return;
        try {
            const resp = await fetch('/api/endpoints/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: id })
            });
            if (resp.ok) {
                showToast('Endpoint deleted');
                updateDashboard();
            } else {
                const text = await resp.text();
                showToast('Failed: ' + text, 'error');
            }
        } catch (err) {
            showToast('Failed to delete', 'error');
        }
    } else if (action === 'enable' || action === 'disable') {
        try {
            const resp = await fetch('/api/endpoints/' + action, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: id })
            });
            if (resp.ok) {
                showToast('Endpoint ' + action + 'd');
                updateDashboard();
            } else {
                showToast('Failed to ' + action, 'error');
            }
        } catch (err) {
            showToast('Failed to ' + action, 'error');
        }
    } else if (action === 'suppress' || action === 'unsuppress') {
        try {
            const resp = await fetch('/api/endpoints/' + action, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: id })
            });
            if (resp.ok) {
                showToast(action === 'suppress' ? 'Alerts suppressed' : 'Alerts enabled');
                updateDashboard();
            } else {
                showToast('Failed to update alerts', 'error');
            }
        } catch (err) {
            showToast('Failed to update alerts', 'error');
        }
    } else if (action === 'edit') {
        openEditModal(id, name, actionsDiv.dataset.interval, actionsDiv.dataset.timeout,
            actionsDiv.dataset.failure, actionsDiv.dataset.success);
    } else if (action === 'history') {
        openHistoryModal(id, name);
    } else if (action === 'enable-health') {
        openEnableHealthModal(id, name);
    }
});

// Enable Health Monitoring Modal
function openEnableHealthModal(id, name) {
    document.getElementById('enable-health-id').value = id;
    document.getElementById('enable-health-name').textContent = name;
    document.getElementById('enable-health-passkey').value = '';
    document.getElementById('enableHealthModal').classList.add('active');
}

function closeEnableHealthModal() {
    document.getElementById('enableHealthModal').classList.remove('active');
}

async function enableHealthMonitoring(e) {
    e.preventDefault();

    const id = document.getElementById('enable-health-id').value;
    const passkey = document.getElementById('enable-health-passkey').value;
    const interval = document.getElementById('enable-health-interval').value || '30s';
    const timeout = document.getElementById('enable-health-timeout').value || '10s';
    const expectedStatus = parseInt(document.getElementById('enable-health-status').value) || 200;
    const failureThreshold = parseInt(document.getElementById('enable-health-failure').value) || 3;
    const successThreshold = parseInt(document.getElementById('enable-health-success').value) || 2;

    try {
        const resp = await fetch('/api/endpoints/enable-health', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id: id,
                passkey: passkey,
                check_interval: interval,
                timeout: timeout,
                expected_status: expectedStatus,
                failure_threshold: failureThreshold,
                success_threshold: successThreshold
            })
        });

        if (resp.ok) {
            showToast('Health monitoring enabled');
            closeEnableHealthModal();
            updateDashboard();
        } else {
            const text = await resp.text();
            if (resp.status === 401) {
                showToast('Invalid passkey', 'error');
            } else {
                showToast('Failed: ' + text, 'error');
            }
        }
    } catch (err) {
        showToast('Failed to enable health monitoring', 'error');
    }
}

function openEditModal(id, name, interval, timeout, failure, success) {
    document.getElementById('edit-id').value = id;
    document.getElementById('edit-name').textContent = name;
    document.getElementById('edit-interval').value = interval || '30s';
    document.getElementById('edit-timeout').value = timeout || '10s';
    document.getElementById('edit-failure').value = failure || 3;
    document.getElementById('edit-success').value = success || 2;
    document.getElementById('editModal').classList.add('active');
}

function closeEditModal() {
    document.getElementById('editModal').classList.remove('active');
}

async function updateEndpoint(e) {
    e.preventDefault();
    const data = {
        id: document.getElementById('edit-id').value,
        check_interval: document.getElementById('edit-interval').value,
        timeout: document.getElementById('edit-timeout').value,
        failure_threshold: parseInt(document.getElementById('edit-failure').value) || 3,
        success_threshold: parseInt(document.getElementById('edit-success').value) || 2
    };
    try {
        const resp = await fetch('/api/endpoints/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (resp.ok) {
            showToast('Endpoint updated');
            closeEditModal();
            updateDashboard();
        } else {
            const err = await resp.text();
            showToast(err, 'error');
        }
    } catch (err) {
        showToast('Failed to update', 'error');
    }
}

async function openHistoryModal(id, name) {
    document.getElementById('history-name').textContent = name;
    document.getElementById('historyModal').classList.add('active');

    try {
        const resp = await fetch('/api/history?id=' + id);
        if (!resp.ok) return;
        const data = await resp.json();
        const records = data.records || [];

        // Calculate stats
        let healthy = 0, unhealthy = 0;
        records.forEach(r => {
            if (r.status === 'healthy') healthy++;
            else if (r.status === 'unhealthy') unhealthy++;
        });
        const total = records.length;
        const uptime = total > 0 ? ((healthy / total) * 100).toFixed(1) : 0;

        document.getElementById('hist-total').textContent = total;
        document.getElementById('hist-healthy').textContent = healthy;
        document.getElementById('hist-unhealthy').textContent = unhealthy;
        document.getElementById('hist-uptime').textContent = uptime + '%';
        document.getElementById('hist-avg').textContent = data.avg_response_time_ms ? formatDuration(data.avg_response_time_ms) : '-';

        // Status timeline chart
        const chartEl = document.getElementById('history-chart-large');
        chartEl.innerHTML = '';
        const displayRecords = records.slice(0, 2000).reverse();
        const tooltip = document.getElementById('chart-tooltip');
        displayRecords.forEach(r => {
            const bar = document.createElement('div');
            bar.style.cssText = 'flex:1;min-width:1px;max-width:3px;border-radius:1px 1px 0 0;cursor:pointer;';
            bar.style.background = r.status === 'healthy' ? '#10b981' : r.status === 'unhealthy' ? '#ef4444' : '#9ca3af';
            bar.style.height = '100%';
            const respTime = r.response_time ? formatDuration(r.response_time / 1000000) : '-';
            bar.onmouseenter = function (e) {
                tooltip.innerHTML = '<strong>' + r.status + '</strong><br>' + respTime + '<br>' + new Date(r.timestamp).toLocaleString();
                tooltip.style.display = 'block';
                tooltip.style.left = (e.clientX + 10) + 'px';
                tooltip.style.top = (e.clientY - 60) + 'px';
            };
            bar.onmousemove = function (e) {
                tooltip.style.left = (e.clientX + 10) + 'px';
                tooltip.style.top = (e.clientY - 60) + 'px';
            };
            bar.onmouseleave = function () {
                tooltip.style.display = 'none';
            };
            chartEl.appendChild(bar);
        });

        // Add X-axis labels for Status Timeline
        const timelineXAxis = document.getElementById('timeline-x-axis');
        timelineXAxis.innerHTML = '';
        if (displayRecords.length > 0) {
            const numLabels = 5;
            for (let i = 0; i < numLabels; i++) {
                const idx = Math.floor(i * (displayRecords.length - 1) / (numLabels - 1));
                const record = displayRecords[idx];
                const label = document.createElement('span');
                label.textContent = new Date(record.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                timelineXAxis.appendChild(label);
            }
        }

        // Response time line chart
        const canvas = document.getElementById('response-chart');
        const ctx = canvas.getContext('2d');
        const rect = canvas.parentElement.getBoundingClientRect();
        canvas.width = rect.width - 20;
        canvas.height = rect.height - 20;

        const responseTimes = displayRecords.map(r => r.response_time ? r.response_time / 1000000 : 0);
        const maxTime = Math.max(...responseTimes, 1);
        const padding = 40;
        const chartWidth = canvas.width - padding * 2;
        const chartHeight = canvas.height - 30;

        // Draw grid lines
        ctx.strokeStyle = '#e5e7eb';
        ctx.lineWidth = 1;
        for (let i = 0; i <= 4; i++) {
            const y = 10 + (chartHeight / 4) * i;
            ctx.beginPath();
            ctx.moveTo(padding, y);
            ctx.lineTo(canvas.width - 10, y);
            ctx.stroke();

            // Y-axis labels
            ctx.fillStyle = '#6b7280';
            ctx.font = '10px sans-serif';
            ctx.textAlign = 'right';
            const val = Math.round(maxTime - (maxTime / 4) * i);
            ctx.fillText(val + 'ms', padding - 5, y + 3);
        }

        // Draw X-axis labels for Response Time chart
        ctx.fillStyle = '#6b7280';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'center';
        if (displayRecords.length > 0) {
            const numXLabels = 5;
            for (let i = 0; i < numXLabels; i++) {
                const idx = Math.floor(i * (displayRecords.length - 1) / (numXLabels - 1));
                const record = displayRecords[idx];
                const x = padding + (idx / (displayRecords.length - 1)) * chartWidth;
                const timeStr = new Date(record.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                ctx.fillText(timeStr, x, chartHeight + 25);
            }
        }

        // Draw line chart
        if (responseTimes.length > 1) {
            ctx.beginPath();
            ctx.strokeStyle = '#6366f1';
            ctx.lineWidth = 2;

            responseTimes.forEach((time, i) => {
                const x = padding + (i / (responseTimes.length - 1)) * chartWidth;
                const y = 10 + chartHeight - (time / maxTime) * chartHeight;
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            // Draw area fill
            ctx.lineTo(padding + chartWidth, 10 + chartHeight);
            ctx.lineTo(padding, 10 + chartHeight);
            ctx.closePath();
            ctx.fillStyle = 'rgba(99, 102, 241, 0.1)';
            ctx.fill();

            // Draw dots for unhealthy points
            displayRecords.forEach((r, i) => {
                if (r.status === 'unhealthy') {
                    const x = padding + (i / (responseTimes.length - 1)) * chartWidth;
                    const time = r.response_time ? r.response_time / 1000000 : 0;
                    const y = 10 + chartHeight - (time / maxTime) * chartHeight;
                    ctx.beginPath();
                    ctx.arc(x, y, 4, 0, Math.PI * 2);
                    ctx.fillStyle = '#ef4444';
                    ctx.fill();
                }
            });

            // Add hover tooltip for response time chart
            const tooltip = document.getElementById('chart-tooltip');
            canvas.onmousemove = function (e) {
                const canvasRect = canvas.getBoundingClientRect();
                const mouseX = e.clientX - canvasRect.left;
                const idx = Math.round(((mouseX - padding) / chartWidth) * (displayRecords.length - 1));
                if (idx >= 0 && idx < displayRecords.length) {
                    const r = displayRecords[idx];
                    const respTime = r.response_time ? formatDuration(r.response_time / 1000000) : '-';
                    tooltip.innerHTML = '<strong>' + r.status + '</strong><br>' + respTime + '<br>' + new Date(r.timestamp).toLocaleString();
                    tooltip.style.display = 'block';
                    tooltip.style.left = (e.clientX + 10) + 'px';
                    tooltip.style.top = (e.clientY - 60) + 'px';
                }
            };
            canvas.onmouseleave = function () {
                tooltip.style.display = 'none';
            };
        }
    } catch (err) {
        console.error('Error loading history:', err);
    }
}

function closeHistoryModal() {
    document.getElementById('historyModal').classList.remove('active');
}

async function openExpiringCertsModal() {
    document.getElementById('expiringCertsModal').classList.add('active');

    try {
        const resp = await fetch('/api/expiring-certs');
        if (!resp.ok) {
            document.getElementById('expiring-certs-list').innerHTML = '<div style="color:#ef4444;padding:20px;text-align:center;">Failed to load expiring certificates</div>';
            return;
        }

        const data = await resp.json();
        const certs = data.expiring_certs || [];
        const listEl = document.getElementById('expiring-certs-list');

        if (certs.length === 0) {
            listEl.innerHTML = '<div style="color:#6b7280;padding:20px;text-align:center;">No SSL certificates expiring within ' + appConfig.ssl_expiry_warning_days + ' days</div>';
            return;
        }

        // Create table
        let html = '<table style="width:100%;border-collapse:collapse;">';
        html += '<thead><tr style="background:#f9fafb;border-bottom:2px solid #e5e7eb;">';
        html += '<th style="padding:12px;text-align:left;font-weight:600;color:#374151;">Endpoint</th>';
        html += '<th style="padding:12px;text-align:left;font-weight:600;color:#374151;">URL</th>';
        html += '<th style="padding:12px;text-align:center;font-weight:600;color:#374151;">Expiry Date</th>';
        html += '<th style="padding:12px;text-align:center;font-weight:600;color:#374151;">Days Remaining</th>';
        html += '</tr></thead><tbody>';

        certs.forEach(cert => {
            const expiryDate = cert.expiry_date ? new Date(cert.expiry_date).toLocaleDateString() : '-';
            const daysColor = cert.days_to_expiry <= 7 ? '#ef4444' : cert.days_to_expiry <= 14 ? '#f59e0b' : '#6b7280';

            html += '<tr style="border-bottom:1px solid #e5e7eb;">';
            html += `<td style="padding:12px;color:#374151;font-weight:500;">${cert.name}</td>`;
            html += `<td style="padding:12px;color:#6366f1;font-family:monospace;font-size:0.85em;">${cert.url}</td>`;
            html += `<td style="padding:12px;text-align:center;color:#374151;">${expiryDate}</td>`;
            html += `<td style="padding:12px;text-align:center;font-weight:600;color:${daysColor};">${cert.days_to_expiry} days</td>`;
            html += '</tr>';
        });

        html += '</tbody></table>';
        listEl.innerHTML = html;
    } catch (err) {
        console.error('Error loading expiring certs:', err);
        document.getElementById('expiring-certs-list').innerHTML = '<div style="color:#ef4444;padding:20px;text-align:center;">Error loading certificates</div>';
    }
}

function closeExpiringCertsModal() {
    document.getElementById('expiringCertsModal').classList.remove('active');
}

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', async function () {
    // Load config first
    await loadConfig();

    // Set 'all' filter as active
    currentFilter = 'all';
    document.querySelectorAll('.stat-card').forEach(card => {
        if (card.dataset.filter === 'all') {
            card.classList.add('active');
        }
    });

    // Update SSL monitoring info in add form
    const sslInfoEl = document.getElementById('ssl-monitoring-info');
    if (sslInfoEl) {
        sslInfoEl.textContent = 'SSL certificate expiry monitored (warning at ' + appConfig.ssl_expiry_warning_days + ' days)';
    }

    updateDashboard();
    setInterval(updateDashboard, 30000);
});
