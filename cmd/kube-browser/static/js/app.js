const state = {
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
    const nsSelect = $('#namespace-select');
    const statusEl = $('#status-indicator');

    if (statusEl.classList.contains('connected')) {
        loadNamespaces();
    }

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
});
