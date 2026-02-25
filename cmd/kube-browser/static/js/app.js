const state = {
    connected: false,
    namespace: '',
    pvc: '',
    currentPath: '/',
    files: [],
};

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

function showToast(message, type = 'info') {
    const container = $('#toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => toast.remove(), 4000);
}

async function api(url, options = {}) {
    try {
        const res = await fetch(url, options);
        const data = await res.json();
        if (!res.ok) {
            throw new Error(data.error || 'Request failed');
        }
        return data;
    } catch (err) {
        showToast(err.message, 'error');
        throw err;
    }
}

function setConnected(connected) {
    state.connected = connected;
    const indicator = $('#status-indicator');
    const text = $('#status-text');
    indicator.className = `status ${connected ? 'connected' : 'disconnected'}`;
    text.textContent = connected ? 'Connected' : 'Disconnected';

    const mainContent = $('#main-content');
    const connectionModal = $('#connection-modal');

    if (connected) {
        mainContent.classList.remove('hidden');
        connectionModal.classList.add('hidden');
    } else {
        mainContent.classList.add('hidden');
        connectionModal.classList.remove('hidden');
    }
}

async function loadKubeconfig() {
    const pathInput = $('#kubeconfig-path');
    const contextSelect = $('#context-select');
    const nsSelect = $('#connect-namespace-select');
    const connectBtn = $('#connect-btn');
    const errorDiv = $('#connection-error');

    errorDiv.classList.add('hidden');
    contextSelect.disabled = true;
    nsSelect.disabled = true;
    connectBtn.disabled = true;
    contextSelect.innerHTML = '<option value="">Loading...</option>';

    try {
        const data = await api('/api/kubeconfig', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: pathInput.value }),
        });

        contextSelect.innerHTML = '';
        if (data.contexts && data.contexts.length > 0) {
            data.contexts.forEach(ctx => {
                const opt = document.createElement('option');
                opt.value = ctx.name;
                opt.textContent = ctx.name;
                if (ctx.name === data.current) opt.selected = true;
                contextSelect.appendChild(opt);
            });
            contextSelect.disabled = false;
            connectBtn.disabled = false;

            nsSelect.innerHTML = '<option value="">All namespaces</option>';
            const selectedCtx = data.contexts.find(c => c.name === data.current) || data.contexts[0];
            if (selectedCtx && selectedCtx.namespace) {
                const opt = document.createElement('option');
                opt.value = selectedCtx.namespace;
                opt.textContent = selectedCtx.namespace;
                opt.selected = true;
                nsSelect.appendChild(opt);
            }
            nsSelect.disabled = false;
        } else {
            contextSelect.innerHTML = '<option value="">No contexts found</option>';
        }
    } catch (e) {
        contextSelect.innerHTML = '<option value="">No contexts</option>';
        errorDiv.textContent = e.message;
        errorDiv.classList.remove('hidden');
    }
}

async function connect() {
    const kubeconfigPath = $('#kubeconfig-path').value;
    const context = $('#context-select').value;
    const errorDiv = $('#connection-error');
    const connectBtn = $('#connect-btn');

    errorDiv.classList.add('hidden');
    connectBtn.disabled = true;
    connectBtn.innerHTML = '<div class="spinner" style="width:16px;height:16px;border-width:2px"></div> Connecting...';

    try {
        const data = await api('/api/connect', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                kubeconfigPath: kubeconfigPath,
                context: context,
            }),
        });

        setConnected(true);
        showToast('Connected to Kubernetes cluster', 'success');

        const nsSelect = $('#namespace-select');
        nsSelect.innerHTML = '<option value="">Select namespace...</option>';
        if (data.namespaces) {
            data.namespaces.forEach(ns => {
                const opt = document.createElement('option');
                opt.value = ns;
                opt.textContent = ns;
                nsSelect.appendChild(opt);
            });
        }

        const selectedNs = $('#connect-namespace-select').value;
        if (selectedNs) {
            nsSelect.value = selectedNs;
            state.namespace = selectedNs;
            loadPVCs(selectedNs);
        }

        $('#disconnect-btn').classList.remove('hidden');
    } catch (e) {
        errorDiv.textContent = e.message;
        errorDiv.classList.remove('hidden');
    } finally {
        connectBtn.disabled = false;
        connectBtn.innerHTML = `<svg viewBox="0 0 20 20" width="16" height="16" fill="currentColor">
            <path d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16zm1-11a1 1 0 1 0-2 0v3.586L7.707 9.293a1 1 0 0 0-1.414 1.414l3 3a1 1 0 0 0 1.414 0l3-3a1 1 0 0 0-1.414-1.414L11 10.586V7z"/>
        </svg> Connect`;
    }
}

async function disconnect() {
    try {
        await api('/api/disconnect', { method: 'POST' });
        setConnected(false);
        showToast('Disconnected from cluster', 'info');

        state.namespace = '';
        state.pvc = '';
        state.currentPath = '/';

        $('#namespace-select').innerHTML = '<option value="">Select namespace...</option>';
        $('#pvc-list').innerHTML = '<div class="empty-state">Select a namespace</div>';
        $('#upload-btn').disabled = true;
        $('#refresh-btn').disabled = true;

        $('#disconnect-btn').classList.add('hidden');
        $('#connect-btn').classList.remove('hidden');
    } catch (e) {
        showToast('Failed to disconnect', 'error');
    }
}

async function loadNamespaces() {
    const select = $('#namespace-select');
    try {
        const data = await api('/api/namespaces');
        select.innerHTML = '<option value="">Select namespace...</option>';
        if (data.namespaces) {
            data.namespaces.forEach(ns => {
                const opt = document.createElement('option');
                opt.value = ns;
                opt.textContent = ns;
                select.appendChild(opt);
            });
        }
    } catch (e) {
        select.innerHTML = '<option value="">Failed to load</option>';
    }
}

async function loadPVCs(namespace) {
    const list = $('#pvc-list');
    list.innerHTML = '<div class="loading"><div class="spinner"></div></div>';

    try {
        const data = await api(`/api/pvcs?namespace=${encodeURIComponent(namespace)}`);
        if (!data.pvcs || data.pvcs.length === 0) {
            list.innerHTML = '<div class="empty-state">No PVCs found</div>';
            return;
        }

        list.innerHTML = '';
        data.pvcs.forEach(pvc => {
            const item = document.createElement('div');
            item.className = 'pvc-item';
            item.dataset.name = pvc.name;

            const statusClass = pvc.status === 'Bound' ? 'bound' : 'pending';
            const mountInfo = pvc.mountedBy ? `Pod: ${pvc.mountedBy}` : 'Not mounted';

            item.innerHTML = `
                <div class="pvc-item-name">${pvc.name}</div>
                <div class="pvc-item-meta">
                    <span class="pvc-status">
                        <span class="pvc-status-dot ${statusClass}"></span>
                        ${pvc.status}
                    </span>
                    <span>${pvc.capacity || 'N/A'}</span>
                </div>
                <div class="pvc-item-meta" style="margin-top:2px">
                    <span>${mountInfo}</span>
                </div>
            `;

            item.addEventListener('click', () => selectPVC(pvc.name));
            list.appendChild(item);
        });
    } catch (e) {
        list.innerHTML = '<div class="empty-state">Failed to load PVCs</div>';
    }
}

function selectPVC(pvcName) {
    state.pvc = pvcName;
    state.currentPath = '/';

    $$('.pvc-item').forEach(item => {
        item.classList.toggle('active', item.dataset.name === pvcName);
    });

    $('#upload-btn').disabled = false;
    $('#refresh-btn').disabled = false;

    loadFiles();
}

async function loadFiles() {
    const container = $('#file-table-container');
    container.innerHTML = '<div class="loading"><div class="spinner"></div></div>';

    try {
        const params = new URLSearchParams({
            namespace: state.namespace,
            pvc: state.pvc,
            path: state.currentPath,
        });

        const data = await api(`/api/files?${params}`);
        renderFiles(data.files || []);
        updateBreadcrumb();
    } catch (e) {
        container.innerHTML = '<div class="empty-state-large"><p>Failed to load files</p></div>';
    }
}

function renderFiles(files) {
    const container = $('#file-table-container');

    if (files.length === 0) {
        container.innerHTML = `
            <div class="empty-state-large">
                <svg viewBox="0 0 64 64" width="48" height="48" fill="none" stroke="#666" stroke-width="2">
                    <rect x="12" y="16" width="40" height="36" rx="3"/>
                    <path d="M20 28h24M20 36h16M20 44h20" stroke-linecap="round"/>
                </svg>
                <p>This directory is empty</p>
            </div>
        `;
        return;
    }

    const folderIcon = `<svg class="file-icon folder" viewBox="0 0 20 20" fill="currentColor"><path d="M2 4a2 2 0 0 1 2-2h3.17a2 2 0 0 1 1.41.59l1.42 1.41H16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V4z"/></svg>`;
    const fileIcon = `<svg class="file-icon file" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4 2a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.414A2 2 0 0 0 17.414 6L14 2.586A2 2 0 0 0 12.586 2H4zm8 1.414L15.586 7H13a1 1 0 0 1-1-1V3.414zM4 4h6v2a3 3 0 0 0 3 3h2v7H4V4z"/></svg>`;

    let html = `
        <table class="file-table">
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Size</th>
                    <th>Modified</th>
                    <th style="text-align:right">Actions</th>
                </tr>
            </thead>
            <tbody>
    `;

    const sortedFiles = [...files].sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
        return a.name.localeCompare(b.name);
    });

    sortedFiles.forEach(file => {
        const icon = file.isDir ? folderIcon : fileIcon;
        const downloadBtn = file.isDir ? '' : `
            <button class="btn btn-secondary" onclick="event.stopPropagation(); downloadFile('${escapeHtml(file.path)}')">
                <svg viewBox="0 0 20 20" width="14" height="14" fill="currentColor">
                    <path d="M10 13l-5-5h3V3h4v5h3l-5 5zM3 16h14v2H3v-2z"/>
                </svg>
                Download
            </button>
        `;

        html += `
            <tr onclick="${file.isDir ? `navigateTo('${escapeHtml(file.path)}')` : ''}">
                <td>
                    <div class="file-name">
                        ${icon}
                        <span>${escapeHtml(file.name)}</span>
                    </div>
                </td>
                <td>${file.isDir ? '-' : formatSize(file.size)}</td>
                <td>${file.modTime}</td>
                <td class="file-actions">${downloadBtn}</td>
            </tr>
        `;
    });

    html += '</tbody></table>';
    container.innerHTML = html;
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML.replace(/'/g, "\\'");
}

function formatSize(size) {
    const num = parseInt(size);
    if (isNaN(num)) return size;
    if (num < 1024) return num + ' B';
    if (num < 1024 * 1024) return (num / 1024).toFixed(1) + ' KB';
    if (num < 1024 * 1024 * 1024) return (num / (1024 * 1024)).toFixed(1) + ' MB';
    return (num / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
}

function updateBreadcrumb() {
    const breadcrumb = $('#breadcrumb');
    const parts = state.currentPath.split('/').filter(Boolean);

    let html = `<span class="breadcrumb-item clickable" onclick="navigateTo('/')">${state.pvc}</span>`;

    let currentPath = '';
    parts.forEach((part, i) => {
        currentPath += '/' + part;
        html += `<span class="breadcrumb-separator">/</span>`;
        if (i === parts.length - 1) {
            html += `<span class="breadcrumb-item">${escapeHtml(part)}</span>`;
        } else {
            html += `<span class="breadcrumb-item clickable" onclick="navigateTo('${escapeHtml(currentPath)}')">${escapeHtml(part)}</span>`;
        }
    });

    breadcrumb.innerHTML = html;
}

function navigateTo(path) {
    state.currentPath = path;
    loadFiles();
}

function downloadFile(filePath) {
    const params = new URLSearchParams({
        namespace: state.namespace,
        pvc: state.pvc,
        path: filePath,
    });
    window.location.href = `/api/download?${params}`;
}

function initUpload() {
    const modal = $('#upload-modal');
    const zone = $('#upload-zone');
    const input = $('#file-input');
    const closeBtn = $('#upload-modal-close');

    $('#upload-btn').addEventListener('click', () => {
        modal.classList.remove('hidden');
        $('#upload-progress').classList.add('hidden');
    });

    closeBtn.addEventListener('click', () => {
        modal.classList.add('hidden');
    });

    modal.addEventListener('click', (e) => {
        if (e.target === modal) modal.classList.add('hidden');
    });

    zone.addEventListener('click', () => input.click());

    zone.addEventListener('dragover', (e) => {
        e.preventDefault();
        zone.classList.add('dragover');
    });

    zone.addEventListener('dragleave', () => {
        zone.classList.remove('dragover');
    });

    zone.addEventListener('drop', (e) => {
        e.preventDefault();
        zone.classList.remove('dragover');
        if (e.dataTransfer.files.length > 0) {
            uploadFile(e.dataTransfer.files[0]);
        }
    });

    input.addEventListener('change', () => {
        if (input.files.length > 0) {
            uploadFile(input.files[0]);
        }
    });
}

async function uploadFile(file) {
    const progress = $('#upload-progress');
    const progressFill = $('#progress-fill');
    const statusText = $('#upload-status');

    progress.classList.remove('hidden');
    progressFill.style.width = '0%';
    statusText.textContent = `Uploading ${file.name}...`;

    const formData = new FormData();
    formData.append('file', file);
    formData.append('namespace', state.namespace);
    formData.append('pvc', state.pvc);
    formData.append('path', state.currentPath);

    try {
        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener('progress', (e) => {
            if (e.lengthComputable) {
                const pct = Math.round((e.loaded / e.total) * 100);
                progressFill.style.width = pct + '%';
                statusText.textContent = `Uploading ${file.name}... ${pct}%`;
            }
        });

        await new Promise((resolve, reject) => {
            xhr.onload = () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    resolve(JSON.parse(xhr.responseText));
                } else {
                    const data = JSON.parse(xhr.responseText);
                    reject(new Error(data.error || 'Upload failed'));
                }
            };
            xhr.onerror = () => reject(new Error('Upload failed'));
            xhr.open('POST', '/api/upload');
            xhr.send(formData);
        });

        progressFill.style.width = '100%';
        statusText.textContent = `${file.name} uploaded successfully!`;
        showToast(`${file.name} uploaded successfully`, 'success');

        setTimeout(() => {
            $('#upload-modal').classList.add('hidden');
            loadFiles();
        }, 1500);
    } catch (err) {
        statusText.textContent = `Failed: ${err.message}`;
        showToast(`Upload failed: ${err.message}`, 'error');
    }
}

document.addEventListener('DOMContentLoaded', () => {
    $('#load-kubeconfig-btn').addEventListener('click', loadKubeconfig);

    $('#kubeconfig-path').addEventListener('keydown', (e) => {
        if (e.key === 'Enter') loadKubeconfig();
    });

    $('#connect-btn').addEventListener('click', connect);
    $('#disconnect-btn').addEventListener('click', disconnect);

    $('#connection-btn').addEventListener('click', () => {
        const modal = $('#connection-modal');
        modal.classList.toggle('hidden');
    });

    const nsSelect = $('#namespace-select');
    nsSelect.addEventListener('change', (e) => {
        state.namespace = e.target.value;
        state.pvc = '';
        state.currentPath = '/';
        $('#upload-btn').disabled = true;
        $('#refresh-btn').disabled = true;
        $('#file-table-container').innerHTML = `
            <div class="empty-state-large">
                <svg viewBox="0 0 64 64" width="64" height="64" fill="none" stroke="#666" stroke-width="2">
                    <rect x="8" y="12" width="48" height="44" rx="3"/>
                    <path d="M8 22h48M20 12V8h24v4"/>
                    <path d="M24 34h16M24 42h10" stroke-linecap="round"/>
                </svg>
                <p>Select a PVC to browse files</p>
            </div>
        `;

        if (state.namespace) {
            loadPVCs(state.namespace);
        } else {
            $('#pvc-list').innerHTML = '<div class="empty-state">Select a namespace</div>';
        }
    });

    $('#refresh-btn').addEventListener('click', () => {
        if (state.pvc) loadFiles();
    });

    initUpload();

    loadKubeconfig();
});
