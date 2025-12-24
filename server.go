package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

// Server provides HTTP endpoints for monitoring status
type Server struct {
	monitor *Monitor
	db      *Database
	port    int
}

// NewServer creates a new HTTP server
func NewServer(monitor *Monitor, db *Database, port int) *Server {
	return &Server{
		monitor: monitor,
		db:      db,
		port:    port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() {
	http.HandleFunc("/", s.handleDashboard)
	http.HandleFunc("/api/status", s.handleAPIStatus)
	http.HandleFunc("/api/health", s.handleHealth)
	http.HandleFunc("/api/endpoints", s.handleEndpoints)
	http.HandleFunc("/api/endpoints/add", s.handleAddEndpoint)
	http.HandleFunc("/api/endpoints/delete", s.handleDeleteEndpoint)
	http.HandleFunc("/api/endpoints/enable", s.handleEnableEndpoint)
	http.HandleFunc("/api/endpoints/disable", s.handleDisableEndpoint)
	http.HandleFunc("/api/endpoints/suppress", s.handleSuppressAlerts)
	http.HandleFunc("/api/endpoints/unsuppress", s.handleUnsuppressAlerts)
	http.HandleFunc("/api/history", s.handleHistory)
	http.HandleFunc("/api/endpoints/update", s.handleUpdateEndpoint)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting web dashboard on http://localhost%s", addr)
	
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()
}

// handleDashboard serves the main dashboard HTML
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Site Watch</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1200px; margin: 0 auto; }
        .header {
            background: white;
            border-radius: 10px;
            padding: 30px;
            margin-bottom: 20px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .header h1 { color: #333; font-size: 2em; margin-bottom: 5px; }
        .header p { color: #666; font-size: 1em; }
        .btn {
            padding: 10px 20px;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-size: 0.9em;
            font-weight: 600;
            transition: all 0.2s;
        }
        .btn-primary { background: #6366f1; color: white; }
        .btn-primary:hover { background: #4f46e5; }
        .btn-success { background: #10b981; color: white; }
        .btn-success:hover { background: #059669; }
        .btn-warning { background: #f59e0b; color: white; }
        .btn-warning:hover { background: #d97706; }
        .btn-danger { background: #ef4444; color: white; }
        .btn-danger:hover { background: #dc2626; }
        .btn-secondary { background: #6b7280; color: white; }
        .btn-secondary:hover { background: #4b5563; }
        .btn-sm { padding: 6px 12px; font-size: 0.8em; }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        .stat-card {
            background: white;
            border-radius: 10px;
            padding: 15px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        .stat-card h3 { color: #666; font-size: 0.8em; text-transform: uppercase; margin-bottom: 5px; }
        .stat-card .value { font-size: 1.8em; font-weight: bold; color: #333; }
        .stat-card.healthy .value { color: #10b981; }
        .stat-card.unhealthy .value { color: #ef4444; }
        .endpoints { display: flex; flex-direction: column; gap: 6px; }
        .endpoint-row {
            background: white;
            border-radius: 6px;
            padding: 8px 12px;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
            display: flex;
            align-items: center;
            gap: 12px;
            font-size: 0.85em;
        }
        .endpoint-row.disabled { opacity: 0.6; background: #f3f4f6; }
        .endpoint-row.unhealthy { border-left: 3px solid #ef4444; }
        .endpoint-row.healthy { border-left: 3px solid #10b981; }
        .endpoint-status { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
        .endpoint-status.healthy { background: #10b981; }
        .endpoint-status.unhealthy { background: #ef4444; }
        .endpoint-status.unknown { background: #9ca3af; }
        .endpoint-name { font-weight: 600; color: #333; min-width: 120px; max-width: 150px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
        .endpoint-url { color: #6366f1; font-family: monospace; font-size: 0.8em; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 150px; }
        .endpoint-stats { display: flex; gap: 12px; align-items: center; color: #6b7280; font-size: 0.8em; }
        .endpoint-stats span { white-space: nowrap; }
        .stat-success { color: #10b981; }
        .stat-fail { color: #ef4444; }
        .stat-avg { color: #6366f1; }
        .endpoint-actions { display: flex; gap: 4px; align-items: center; flex-shrink: 0; }
        .icon-btn {
            width: 28px; height: 28px;
            border: none; border-radius: 6px;
            cursor: pointer; display: flex;
            align-items: center; justify-content: center;
            transition: all 0.2s; font-size: 14px;
        }
        .icon-btn:hover { transform: scale(1.1); }
        .icon-btn.edit { background: #e0e7ff; color: #4f46e5; }
        .icon-btn.toggle-on { background: #fef3c7; color: #d97706; }
        .icon-btn.toggle-off { background: #d1fae5; color: #059669; }
        .icon-btn.alert-on { background: #d1fae5; color: #059669; }
        .icon-btn.alert-off { background: #fef3c7; color: #d97706; }
        .icon-btn.delete { background: #fee2e2; color: #dc2626; }
        .icon-btn.delete:hover { background: #fecaca; }
        .history-mini { display: flex; gap: 1px; align-items: flex-end; height: 16px; }
        .history-mini .bar { width: 3px; border-radius: 1px; }
        .history-mini .bar.success { background: #10b981; height: 100%; }
        .history-mini .bar.failure { background: #ef4444; height: 100%; }
        .history-mini .bar.unknown { background: #9ca3af; height: 50%; }
        .error-message {
            background: #fef2f2;
            border-left: 4px solid #ef4444;
            padding: 10px;
            margin-top: 10px;
            border-radius: 4px;
            color: #991b1b;
            font-size: 0.85em;
        }
        .refresh-info { text-align: center; color: white; margin-top: 20px; font-size: 0.9em; }
        .loading { text-align: center; padding: 40px; color: white; font-size: 1.2em; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        .pulse { animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite; }
        
        /* Modal styles */
        .modal { display: none; position: fixed; z-index: 1000; left: 0; top: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); }
        .modal.active { display: flex; align-items: center; justify-content: center; }
        .modal-content {
            background: white;
            padding: 30px;
            border-radius: 12px;
            width: 90%;
            max-width: 500px;
            max-height: 90vh;
            overflow-y: auto;
        }
        .modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .modal-header h2 { color: #333; font-size: 1.5em; }
        .modal-close { background: none; border: none; font-size: 1.5em; cursor: pointer; color: #666; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; color: #374151; font-weight: 500; }
        .form-group input, .form-group select {
            width: 100%;
            padding: 10px;
            border: 1px solid #d1d5db;
            border-radius: 6px;
            font-size: 1em;
        }
        .form-group input:focus, .form-group select:focus { outline: none; border-color: #6366f1; }
        .form-actions { display: flex; gap: 10px; justify-content: flex-end; margin-top: 20px; }
        .toast {
            position: fixed;
            bottom: 20px;
            right: 20px;
            padding: 15px 25px;
            border-radius: 8px;
            color: white;
            font-weight: 500;
            z-index: 2000;
            animation: slideIn 0.3s ease;
        }
        .toast.success { background: #10b981; }
        .toast.error { background: #ef4444; }
        @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
        
        /* History chart styles */
        .history-chart {
            height: 24px;
            display: flex;
            align-items: flex-end;
            gap: 1px;
            padding: 4px;
            margin: 4px 0;
            background: #f9fafb;
            border-radius: 4px;
            overflow: hidden;
        }
        .history-bar {
            flex: 1;
            min-width: 2px;
            max-width: 4px;
            border-radius: 1px 1px 0 0;
        }
        .history-bar.success { background: #10b981; }
        .history-bar.failure { background: #ef4444; }
        .history-bar.unknown { background: #9ca3af; }
        .success-count .detail-value { color: #10b981; }
        .failure-count .detail-value { color: #ef4444; }
        .avg-response { color: #6366f1; }
        .editable { cursor: pointer; border-bottom: 1px dashed #6366f1; }
        .editable:hover { background: #eef2ff; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div>
                <h1>Site Watch</h1>
                <p>Real-time application health monitoring</p>
            </div>
            <button class="btn btn-primary" onclick="openAddModal()">+ Add Endpoint</button>
        </div>
        
        <div class="stats" id="stats">
            <div class="stat-card"><h3>Total</h3><div class="value" id="total-endpoints">-</div></div>
            <div class="stat-card healthy"><h3>Healthy</h3><div class="value" id="healthy-count">-</div></div>
            <div class="stat-card unhealthy"><h3>Unhealthy</h3><div class="value" id="unhealthy-count">-</div></div>
            <div class="stat-card"><h3>Disabled</h3><div class="value" id="disabled-count">-</div></div>
        </div>
        
        <div class="endpoints" id="endpoints">
            <div class="loading pulse">Loading endpoint status...</div>
        </div>
        
        <div class="refresh-info">Auto-refreshing every 30 seconds • Last updated: <span id="last-update">-</span></div>
    </div>

    <!-- Add Endpoint Modal -->
    <div class="modal" id="addModal">
        <div class="modal-content">
            <div class="modal-header">
                <h2>Add New Endpoint</h2>
                <button class="modal-close" onclick="closeAddModal()">&times;</button>
            </div>
            <form id="addForm" onsubmit="addEndpoint(event)">
                <div class="form-group">
                    <label>Name *</label>
                    <input type="text" id="ep-name" required placeholder="My API">
                </div>
                <div class="form-group">
                    <label>URL *</label>
                    <input type="url" id="ep-url" required placeholder="https://api.example.com/health">
                </div>
                <div class="form-group">
                    <label>Method</label>
                    <select id="ep-method">
                        <option value="GET">GET</option>
                        <option value="POST">POST</option>
                        <option value="HEAD">HEAD</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>Check Interval</label>
                    <input type="text" id="ep-interval" placeholder="30s" value="30s">
                </div>
                <div class="form-group">
                    <label>Timeout</label>
                    <input type="text" id="ep-timeout" placeholder="10s" value="10s">
                </div>
                <div class="form-group">
                    <label>Expected Status Code</label>
                    <input type="number" id="ep-status" placeholder="200" value="200">
                </div>
                <div class="form-group">
                    <label>Failure Threshold</label>
                    <input type="number" id="ep-failure" placeholder="3" value="3">
                </div>
                <div class="form-group">
                    <label>Success Threshold</label>
                    <input type="number" id="ep-success" placeholder="2" value="2">
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeAddModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Add Endpoint</button>
                </div>
            </form>
        </div>
    </div>

    <!-- Edit Endpoint Modal -->
    <div class="modal" id="editModal">
        <div class="modal-content">
            <div class="modal-header">
                <h2>Edit: <span id="edit-name"></span></h2>
                <button class="modal-close" onclick="closeEditModal()">&times;</button>
            </div>
            <form id="editForm" onsubmit="updateEndpoint(event)">
                <input type="hidden" id="edit-id">
                <div class="form-group">
                    <label>Check Interval</label>
                    <input type="text" id="edit-interval" placeholder="30s">
                </div>
                <div class="form-group">
                    <label>Timeout</label>
                    <input type="text" id="edit-timeout" placeholder="10s">
                </div>
                <div class="form-group">
                    <label>Failure Threshold</label>
                    <input type="number" id="edit-failure" placeholder="3">
                </div>
                <div class="form-group">
                    <label>Success Threshold</label>
                    <input type="number" id="edit-success" placeholder="2">
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeEditModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Save</button>
                </div>
            </form>
        </div>
    </div>

    <!-- History Modal -->
    <div class="modal" id="historyModal">
        <div class="modal-content" style="max-width: 900px;">
            <div class="modal-header">
                <h2>History: <span id="history-name"></span></h2>
                <button class="modal-close" onclick="closeHistoryModal()">&times;</button>
            </div>
            <div id="history-stats" style="display:flex;gap:20px;margin-bottom:15px;padding:10px;background:#f9fafb;border-radius:6px;flex-wrap:wrap;">
                <div><strong>Total Checks:</strong> <span id="hist-total">-</span></div>
                <div><strong>Healthy:</strong> <span id="hist-healthy" style="color:#10b981;">-</span></div>
                <div><strong>Unhealthy:</strong> <span id="hist-unhealthy" style="color:#ef4444;">-</span></div>
                <div><strong>Uptime:</strong> <span id="hist-uptime" style="color:#6366f1;">-</span></div>
                <div><strong>Avg Response:</strong> <span id="hist-avg">-</span></div>
            </div>
            <div style="margin-bottom:10px;font-weight:600;color:#374151;">Status Timeline (last 2000 checks)</div>
            <div id="history-chart-large" style="height:80px;display:flex;align-items:flex-end;gap:1px;background:#f9fafb;border-radius:6px;padding:8px;margin-bottom:5px;"></div>
            <div id="timeline-x-axis" style="display:flex;justify-content:space-between;font-size:10px;color:#6b7280;padding:0 8px;margin-bottom:20px;"></div>
            <div style="margin-bottom:10px;font-weight:600;color:#374151;">Response Time Chart (ms)</div>
            <div style="position:relative;height:180px;background:#f9fafb;border-radius:6px;padding:10px;margin-bottom:10px;">
                <canvas id="response-chart" style="width:100%;height:100%;"></canvas>
            </div>
            <div id="chart-tooltip" style="display:none;position:absolute;background:#1f2937;color:white;padding:6px 10px;border-radius:4px;font-size:12px;pointer-events:none;z-index:100;"></div>
        </div>
    </div>

    <script>
        let endpointsData = {};

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
        }

        function closeAddModal() {
            document.getElementById('addModal').classList.remove('active');
            document.getElementById('addForm').reset();
        }

        async function addEndpoint(e) {
            e.preventDefault();
            const data = {
                name: document.getElementById('ep-name').value,
                url: document.getElementById('ep-url').value,
                method: document.getElementById('ep-method').value,
                check_interval: document.getElementById('ep-interval').value,
                timeout: document.getElementById('ep-timeout').value,
                expected_status: parseInt(document.getElementById('ep-status').value) || 200,
                failure_threshold: parseInt(document.getElementById('ep-failure').value) || 3,
                success_threshold: parseInt(document.getElementById('ep-success').value) || 2
            };
            try {
                const resp = await fetch('/api/endpoints/add', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
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
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({id: id})
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
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({id: id})
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
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({id: id})
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

                let healthy = 0, unhealthy = 0, disabled = 0, total = 0;
                
                const endpointsContainer = document.getElementById('endpoints');
                endpointsContainer.innerHTML = '';

                // Combine status data with DB settings
                const allEndpoints = [];
                Object.entries(statusData.endpoints || {}).forEach(([name, endpoint]) => {
                    const dbEp = Object.values(dbEndpoints).find(e => e.name === endpoint.name) || {};
                    allEndpoints.push({...endpoint, ...dbEp, id: endpoint.id || dbEp.id || name});
                });

                // Also add any DB endpoints not in status
                Object.values(dbEndpoints).forEach(dbEp => {
                    if (!allEndpoints.find(e => e.id === dbEp.id)) {
                        allEndpoints.push({...dbEp, status: 'unknown'});
                    }
                });

                allEndpoints.forEach(endpoint => {
                    total++;
                    const isEnabled = endpoint.enabled !== false;
                    const isSuppressed = endpoint.alerts_suppressed === true;
                    
                    if (!isEnabled) disabled++;
                    else if (endpoint.status === 'healthy') healthy++;
                    else if (endpoint.status === 'unhealthy') unhealthy++;

                    const row = document.createElement('div');
                    row.className = 'endpoint-row ' + endpoint.status + (isEnabled ? '' : ' disabled');
                    
                    row.innerHTML = ` + "`" + `
                        <div class="endpoint-status ${endpoint.status}"></div>
                        <div class="endpoint-name" title="${endpoint.name}">${endpoint.name}</div>
                        <div class="endpoint-url" title="${endpoint.url}">${endpoint.url}</div>
                        <div class="history-mini" id="chart-${endpoint.id}"></div>
                        <div class="endpoint-stats">
                            <span title="Response Time">${formatDuration(endpoint.response_time_ms || 0)}</span>
                            <span class="stat-avg" title="Avg Response" id="avg-${endpoint.id}">-</span>
                            <span title="Interval">${formatInterval(endpoint.check_interval)}</span>
                            <span class="stat-success" title="Consecutive Successes">✓${endpoint.consecutive_successes || 0}</span>
                            <span class="stat-fail" title="Consecutive Failures">✗${endpoint.consecutive_failures || 0}</span>
                        </div>
                        <div class="endpoint-actions" data-endpoint-id="${endpoint.id}" data-endpoint-name="${endpoint.name}" 
                             data-interval="${formatInterval(endpoint.check_interval)}" data-timeout="${formatInterval(endpoint.timeout)}"
                             data-failure="${endpoint.failure_threshold || 3}" data-success="${endpoint.success_threshold || 2}">
                            <button class="icon-btn edit" data-action="history" title="View History">📊</button>
                            <button class="icon-btn edit" data-action="edit" title="Edit">✏️</button>
                            <button class="icon-btn ${isEnabled ? 'toggle-on' : 'toggle-off'}" data-action="${isEnabled ? 'disable' : 'enable'}" title="${isEnabled ? 'Disable' : 'Enable'}">${isEnabled ? '⏸️' : '▶️'}</button>
                            <button class="icon-btn ${isSuppressed ? 'alert-on' : 'alert-off'}" data-action="${isSuppressed ? 'unsuppress' : 'suppress'}" title="${isSuppressed ? 'Enable Alerts' : 'Suppress Alerts'}">${isSuppressed ? '🔔' : '🔕'}</button>
                            <button class="icon-btn delete" data-action="delete" title="Delete">🗑️</button>
                        </div>
                    ` + "`" + `;
                    
                    endpointsContainer.appendChild(row);
                    
                    // Load history chart for this endpoint
                    loadHistoryChart(endpoint.id);
                });

                document.getElementById('total-endpoints').textContent = total;
                document.getElementById('healthy-count').textContent = healthy;
                document.getElementById('unhealthy-count').textContent = unhealthy;
                document.getElementById('disabled-count').textContent = disabled;
                document.getElementById('last-update').textContent = new Date().toLocaleTimeString();
            } catch (error) {
                console.error('Error fetching status:', error);
            }
        }

        // Event delegation for action buttons
        document.addEventListener('click', async function(e) {
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
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({id: id})
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
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({id: id})
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
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({id: id})
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
            }
        });

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
                    headers: {'Content-Type': 'application/json'},
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
                    bar.onmouseenter = function(e) {
                        tooltip.innerHTML = '<strong>' + r.status + '</strong><br>' + respTime + '<br>' + new Date(r.timestamp).toLocaleString();
                        tooltip.style.display = 'block';
                        tooltip.style.left = (e.clientX + 10) + 'px';
                        tooltip.style.top = (e.clientY - 60) + 'px';
                    };
                    bar.onmousemove = function(e) {
                        tooltip.style.left = (e.clientX + 10) + 'px';
                        tooltip.style.top = (e.clientY - 60) + 'px';
                    };
                    bar.onmouseleave = function() {
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
                        label.textContent = new Date(record.timestamp).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
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
                        const timeStr = new Date(record.timestamp).toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
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
                    canvas.onmousemove = function(e) {
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
                    canvas.onmouseleave = function() {
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

        updateDashboard();
        setInterval(updateDashboard, 30000);
    </script>
</body>
</html>`

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, nil)
}

// StatusResponse represents the API response for endpoint status
type StatusResponse struct {
	Endpoints map[string]EndpointStatus `json:"endpoints"`
	Timestamp time.Time                 `json:"timestamp"`
}

// EndpointStatus represents the status of a single endpoint for API response
type EndpointStatus struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	URL                  string  `json:"url"`
	Method               string  `json:"method"`
	Status               string  `json:"status"`
	LastCheck            string  `json:"last_check"`
	LastError            string  `json:"last_error"`
	ResponseTimeMs       float64 `json:"response_time_ms"`
	ConsecutiveFailures  int     `json:"consecutive_failures"`
	ConsecutiveSuccesses int     `json:"consecutive_successes"`
}

// handleAPIStatus returns JSON status of all endpoints
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	states := s.monitor.GetStatus()
	
	response := StatusResponse{
		Endpoints: make(map[string]EndpointStatus),
		Timestamp: time.Now(),
	}

	for name, state := range states {
		state.mu.RLock()
		response.Endpoints[name] = EndpointStatus{
			ID:                   state.ID,
			Name:                 state.Endpoint.Name,
			URL:                  state.Endpoint.URL,
			Method:               state.Endpoint.Method,
			Status:               string(state.Status),
			LastCheck:            state.LastCheck.Format(time.RFC3339),
			LastError:            state.LastError,
			ResponseTimeMs:       float64(state.ResponseTime.Microseconds()) / 1000.0,
			ConsecutiveFailures:  state.ConsecutiveFailures,
			ConsecutiveSuccesses: state.ConsecutiveSuccesses,
		}
		state.mu.RUnlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth returns the overall health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	states := s.monitor.GetStatus()
	
	allHealthy := true
	for _, state := range states {
		state.mu.RLock()
		if state.Status == StatusUnhealthy {
			allHealthy = false
		}
		state.mu.RUnlock()
	}

	status := "healthy"
	statusCode := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// EndpointRequest represents a request to add/modify an endpoint
type EndpointRequest struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	URL              string            `json:"url"`
	Method           string            `json:"method"`
	Timeout          string            `json:"timeout"`
	CheckInterval    string            `json:"check_interval"`
	ExpectedStatus   int               `json:"expected_status"`
	Headers          map[string]string `json:"headers"`
	FailureThreshold int               `json:"failure_threshold"`
	SuccessThreshold int               `json:"success_threshold"`
}

// handleEndpoints returns all endpoints from the database
func (s *Server) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := s.db.GetAllEndpoints()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": endpoints,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// handleAddEndpoint adds a new endpoint
func (s *Server) handleAddEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.URL == "" {
		http.Error(w, "Name and URL are required", http.StatusBadRequest)
		return
	}

	// Generate ID from name+URL combination for unique history isolation
	id := generateIDWithURL(req.Name, req.URL)
	
	// Check if endpoint with same name already exists
	allEndpoints, _ := s.db.GetAllEndpoints()
	for _, ep := range allEndpoints {
		if ep.Name == req.Name {
			http.Error(w, "Endpoint with this name already exists", http.StatusConflict)
			return
		}
		if ep.URL == req.URL {
			http.Error(w, "Endpoint with this URL already exists", http.StatusConflict)
			return
		}
	}

	timeout := 10 * time.Second
	if req.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	checkInterval := 30 * time.Second
	if req.CheckInterval != "" {
		var err error
		checkInterval, err = time.ParseDuration(req.CheckInterval)
		if err != nil {
			http.Error(w, "Invalid check_interval format: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	endpoint := &StoredEndpoint{
		ID:               id,
		Name:             req.Name,
		URL:              req.URL,
		Method:           req.Method,
		Timeout:          timeout,
		CheckInterval:    checkInterval,
		ExpectedStatus:   req.ExpectedStatus,
		Headers:          req.Headers,
		FailureThreshold: req.FailureThreshold,
		SuccessThreshold: req.SuccessThreshold,
		Enabled:          true,
		AlertsSuppressed: false,
	}

	if err := s.monitor.AddEndpoint(endpoint); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"endpoint": endpoint,
	})
}

// handleDeleteEndpoint deletes an endpoint
func (s *Server) handleDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	log.Printf("Delete endpoint request: method=%s", r.Method)
	
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		log.Printf("Delete endpoint: method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	log.Printf("Delete endpoint: query id=%s", id)
	
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = req.ID
			log.Printf("Delete endpoint: body id=%s", id)
		} else {
			log.Printf("Delete endpoint: body decode error=%v", err)
		}
	}

	if id == "" {
		log.Printf("Delete endpoint: ID is empty")
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Delete endpoint: attempting to remove id=%s", id)
	if err := s.monitor.RemoveEndpoint(id); err != nil {
		log.Printf("Delete endpoint: error=%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Delete endpoint: success id=%s", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Endpoint deleted",
	})
}

// handleEnableEndpoint enables an endpoint
func (s *Server) handleEnableEndpoint(w http.ResponseWriter, r *http.Request) {
	s.handleEndpointAction(w, r, s.monitor.EnableEndpoint, "enabled")
}

// handleDisableEndpoint disables an endpoint
func (s *Server) handleDisableEndpoint(w http.ResponseWriter, r *http.Request) {
	s.handleEndpointAction(w, r, s.monitor.DisableEndpoint, "disabled")
}

// handleSuppressAlerts suppresses alerts for an endpoint
func (s *Server) handleSuppressAlerts(w http.ResponseWriter, r *http.Request) {
	s.handleEndpointAction(w, r, s.monitor.SuppressAlerts, "alerts suppressed")
}

// handleUnsuppressAlerts enables alerts for an endpoint
func (s *Server) handleUnsuppressAlerts(w http.ResponseWriter, r *http.Request) {
	s.handleEndpointAction(w, r, s.monitor.UnsuppressAlerts, "alerts enabled")
}

// handleEndpointAction is a helper for endpoint actions
func (s *Server) handleEndpointAction(w http.ResponseWriter, r *http.Request, action func(string) error, actionName string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			id = req.ID
		}
	}

	if id == "" {
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	if err := action(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Endpoint " + actionName,
	})
}

// handleHistory returns health check history for an endpoint
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	limit := 1000
	records, err := s.db.GetHealthHistory(id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate average response time
	var totalResponseTime int64
	var count int
	for _, r := range records {
		if r.ResponseTime > 0 {
			totalResponseTime += int64(r.ResponseTime)
			count++
		}
	}
	var avgResponseTimeMs float64
	if count > 0 {
		avgResponseTimeMs = float64(totalResponseTime/int64(count)) / 1000000.0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoint_id":         id,
		"records":             records,
		"avg_response_time_ms": avgResponseTimeMs,
		"record_count":        count,
		"timestamp":           time.Now().Format(time.RFC3339),
	})
}

// handleUpdateEndpoint updates an endpoint's settings
func (s *Server) handleUpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID               string `json:"id"`
		CheckInterval    string `json:"check_interval"`
		Timeout          string `json:"timeout"`
		FailureThreshold int    `json:"failure_threshold"`
		SuccessThreshold int    `json:"success_threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Endpoint ID is required", http.StatusBadRequest)
		return
	}

	// Get existing endpoint
	endpoint, err := s.db.GetEndpoint(req.ID)
	if err != nil {
		http.Error(w, "Endpoint not found: "+err.Error(), http.StatusNotFound)
		return
	}

	// Update fields if provided
	if req.CheckInterval != "" {
		interval, err := time.ParseDuration(req.CheckInterval)
		if err != nil {
			http.Error(w, "Invalid check_interval format: "+err.Error(), http.StatusBadRequest)
			return
		}
		endpoint.CheckInterval = interval
	}
	if req.Timeout != "" {
		timeout, err := time.ParseDuration(req.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format: "+err.Error(), http.StatusBadRequest)
			return
		}
		endpoint.Timeout = timeout
	}
	if req.FailureThreshold > 0 {
		endpoint.FailureThreshold = req.FailureThreshold
	}
	if req.SuccessThreshold > 0 {
		endpoint.SuccessThreshold = req.SuccessThreshold
	}

	// Save to database
	if err := s.db.SaveEndpoint(endpoint); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update monitor state
	s.monitor.UpdateEndpointSettings(req.ID, endpoint)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"endpoint": endpoint,
	})
}
