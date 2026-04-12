(function () {
    'use strict';

    // --- State ---
    var state = {
        config: { concerns: [] },
        pages: [],
        activity: [],
        view: 'dashboard',
        explorePath: '',
        pagePath: '',
        searchResults: [],
        activityOpen: false
    };

    // --- API ---
    function api(path) {
        return fetch(path).then(function (r) {
            if (!r.ok) throw new Error(r.status + ' ' + r.statusText);
            return r.json();
        });
    }

    function loadConfig() { return api('/api/config'); }
    function loadPages() { return api('/api/pages'); }
    function loadPage(path) { return api('/api/pages/' + path); }
    function loadSearch(q) { return api('/api/search?q=' + encodeURIComponent(q)); }
    function loadActivity() { return api('/api/activity'); }

    // --- SSE ---
    function connectSSE() {
        var es = new EventSource('/api/events');
        es.addEventListener('change', function (e) {
            var data = JSON.parse(e.data);
            if (data.kind === 'config') {
                loadConfig().then(function (c) { state.config = c; render(); });
            }
            // Always reload pages on any change
            loadPages().then(function (p) { state.pages = p; render(); });
            loadActivity().then(function (a) { state.activity = a; render(); });
        });
        es.onerror = function () {
            es.close();
            setTimeout(connectSSE, 3000);
        };
    }

    // --- Routing ---
    function parseHash() {
        var hash = location.hash.slice(1) || '/';
        if (hash === '/') return { view: 'dashboard' };
        if (hash === '/explore') return { view: 'explore', path: '' };
        if (hash.indexOf('/explore/') === 0) {
            var rest = hash.slice('/explore/'.length);
            // Check if it ends in .txt — it's a page view
            if (rest.match(/\.txt$/)) return { view: 'page', path: rest };
            return { view: 'explore', path: rest };
        }
        return { view: '404' };
    }

    function navigate() {
        var route = parseHash();
        state.view = route.view;
        state.explorePath = route.path || '';
        state.pagePath = route.path || '';
        render();
        updateNavLinks();
    }

    function updateNavLinks() {
        var links = document.querySelectorAll('.nav-link');
        links.forEach(function (a) {
            a.classList.toggle('active', a.dataset.view === state.view ||
                (a.dataset.view === 'explore' && (state.view === 'explore' || state.view === 'page')));
        });
    }

    // --- Rendering ---
    function render() {
        var el = document.getElementById('content');
        switch (state.view) {
            case 'dashboard': el.innerHTML = renderDashboard(); break;
            case 'explore': el.innerHTML = renderExplore(); break;
            case 'page': renderPageView(el); break;
            case '404': el.innerHTML = render404(); break;
            default: el.innerHTML = render404();
        }
        renderActivityPanel();
    }

    // --- Dashboard ---
    function renderDashboard() {
        var stats = computeStats();
        var html = '<div class="dashboard">';
        html += '<h1>Dashboard</h1>';
        html += '<p class="subtitle">' + esc(state.config.nickname || 'journal') + '</p>';

        html += '<div class="stats-grid">';
        html += statCard(stats.totalPages, 'Pages');
        html += statCard(stats.totalFolders, 'Folders');
        html += statCard(stats.totalBytes, 'Total size');
        html += statCard(stats.concerns, 'Concerns');
        html += '</div>';

        if (state.config.concerns && state.config.concerns.length > 0) {
            html += '<h2 style="font-size:16px;margin-bottom:12px">Concerns</h2>';
            html += '<div class="concerns-list">';
            state.config.concerns.forEach(function (c) {
                var folder = findFolder(state.pages, c.folderName);
                var count = folder ? countFiles(folder.children || []) : 0;
                var kanbanInfo = isKanbanConcern(c, folder);
                html += '<div class="concern-row" onclick="location.hash=\'#/explore/' + esc(c.folderName) + '\'">';
                html += '<span class="concern-icon">' + (kanbanInfo ? '📋' : '📁') + '</span>';
                html += '<div>';
                html += '<div class="concern-name">' + esc(c.label) + '</div>';
                html += '<div class="concern-desc">' + esc(c.description || '') + '</div>';
                if (kanbanInfo) {
                    html += renderKanbanProgress(kanbanInfo);
                }
                html += '</div>';
                html += '<span class="concern-count">' + count + '</span>';
                html += '</div>';
            });
            html += '</div>';
        }

        html += '</div>';
        return html;
    }

    function statCard(value, label) {
        return '<div class="stat-card"><div class="stat-value">' + esc(String(value)) + '</div><div class="stat-label">' + esc(label) + '</div></div>';
    }

    function computeStats() {
        var totalPages = 0, totalFolders = 0;
        function walk(entries) {
            (entries || []).forEach(function (e) {
                if (e.isDir) { totalFolders++; walk(e.children); }
                else { totalPages++; }
            });
        }
        walk(state.pages);

        var bytes = totalPages > 0 ? '—' : '0 B'; // We don't track bytes in tree API
        return {
            totalPages: totalPages,
            totalFolders: totalFolders,
            totalBytes: totalPages + ' files',
            concerns: (state.config.concerns || []).length
        };
    }

    // --- Kanban detection ---
    // A concern is kanban if its config has swimlanes defined,
    // OR if the folder has only subfolders (no direct .txt files at root level).
    function isKanbanConcern(concern, folder) {
        if (!folder || !folder.children || folder.children.length === 0) return null;
        if (concern.swimlanes) return computeKanban(concern, folder);
        // Auto-detect: if all children are dirs, treat as kanban
        var allDirs = folder.children.every(function (c) { return c.isDir; });
        if (!allDirs) return null;
        return computeKanban(concern, folder);
    }

    function computeKanban(concern, folder) {
        var cols = (folder.children || []).filter(function (c) { return c.isDir; });
        if (cols.length < 2) return null;
        // If swimlanes are configured, sort columns to match that order
        if (concern.swimlanes && concern.swimlanes.length > 0) {
            var order = concern.swimlanes;
            cols.sort(function (a, b) {
                var ai = order.indexOf(a.name);
                var bi = order.indexOf(b.name);
                if (ai === -1) ai = 9999;
                if (bi === -1) bi = 9999;
                return ai - bi;
            });
        }
        var total = 0;
        var notStarted = 0, completed = 0, inProgress = 0;
        cols.forEach(function (col, i) {
            var count = countFiles(col.children || []);
            total += count;
            if (i === 0) notStarted = count;
            else if (i === cols.length - 1) completed = count;
            else inProgress += count;
        });
        return { columns: cols, total: total, notStarted: notStarted, inProgress: inProgress, completed: completed };
    }

    function renderKanbanProgress(info) {
        if (info.total === 0) return '';
        var pNot = Math.round(info.notStarted / info.total * 100);
        var pDone = Math.round(info.completed / info.total * 100);
        var pProg = 100 - pNot - pDone;
        return '<div class="kanban-progress">' +
            '<div class="bar-not-started" style="width:' + pNot + '%"></div>' +
            '<div class="bar-in-progress" style="width:' + pProg + '%"></div>' +
            '<div class="bar-completed" style="width:' + pDone + '%"></div>' +
            '</div>';
    }

    // --- Explore ---
    function renderExplore() {
        var path = state.explorePath;
        var entries = resolveEntries(state.pages, path);
        var concern = findConcernForPath(path);

        // Check if this is a kanban view
        if (concern && entries) {
            var folder = { children: entries, isDir: true };
            var kanbanInfo = isKanbanConcern(concern, folder);
            if (kanbanInfo) {
                return renderKanbanView(concern, kanbanInfo, path);
            }
        }

        var html = '<div class="explore">';
        html += '<h1>' + esc(concern ? concern.label : (path || 'Explore')) + '</h1>';
        html += renderBreadcrumb(path);

        if (!entries || entries.length === 0) {
            html += '<p style="color:var(--text-dim)">No pages here yet.</p>';
            html += '</div>';
            return html;
        }

        html += '<ul class="folder-list">';
        entries.forEach(function (e) {
            var target = path ? path + '/' + e.name : e.name;
            if (e.isDir) {
                html += '<li class="folder-item" onclick="location.hash=\'#/explore/' + esc(target) + '\'">';
                html += '<span class="folder-item-icon">📁</span>';
                html += '<span class="folder-item-name">' + esc(e.name) + '</span>';
                html += '<span class="folder-item-preview">' + countFiles(e.children || []) + ' files</span>';
                html += '</li>';
            } else {
                html += '<li class="folder-item" onclick="location.hash=\'#/explore/' + esc(target) + '\'">';
                html += '<span class="folder-item-icon">📄</span>';
                html += '<span class="folder-item-name">' + esc(e.name) + '</span>';
                html += '<span class="folder-item-preview">' + esc(e.preview || '') + '</span>';
                html += '</li>';
            }
        });
        html += '</ul></div>';
        return html;
    }

    function renderBreadcrumb(path) {
        if (!path) return '';
        var parts = path.split('/');
        var html = '<div class="breadcrumb"><a href="#/explore">root</a>';
        var acc = '';
        parts.forEach(function (p, i) {
            acc += (acc ? '/' : '') + p;
            html += ' / ';
            if (i < parts.length - 1) {
                html += '<a href="#/explore/' + esc(acc) + '">' + esc(p) + '</a>';
            } else {
                html += '<span>' + esc(p) + '</span>';
            }
        });
        html += '</div>';
        return html;
    }

    // --- Kanban view ---
    function renderKanbanView(concern, info, path) {
        var html = '<div class="explore">';
        html += '<h1>📋 ' + esc(concern.label) + '</h1>';
        html += renderBreadcrumb(path);
        html += renderKanbanProgress(info);
        html += '<div style="font-size:12px;color:var(--text-dim);margin:8px 0 16px">';
        html += info.notStarted + ' not started · ' + info.inProgress + ' in progress · ' + info.completed + ' completed';
        html += '</div>';

        html += '<div class="kanban-board">';
        info.columns.forEach(function (col, i) {
            var status = '';
            if (i === 0) status = 'Not Started';
            else if (i === info.columns.length - 1) status = 'Completed';
            else status = col.name;
            var files = (col.children || []).filter(function (c) { return !c.isDir; });

            html += '<div class="kanban-column">';
            html += '<div class="kanban-column-header">';
            html += '<span>' + esc(status) + '</span>';
            html += '<span class="kanban-column-count">' + files.length + '</span>';
            html += '</div>';
            html += '<div class="kanban-cards">';
            files.forEach(function (f) {
                var target = path + '/' + col.name + '/' + f.name;
                html += '<div class="kanban-card" onclick="location.hash=\'#/explore/' + esc(target) + '\'">';
                html += '<div class="kanban-card-title">' + esc(f.name.replace(/\.txt$/, '')) + '</div>';
                if (f.preview) html += '<div class="kanban-card-preview">' + esc(f.preview) + '</div>';
                html += '</div>';
            });
            if (files.length === 0) {
                html += '<div style="color:var(--text-dim);font-size:12px;padding:8px;text-align:center">empty</div>';
            }
            html += '</div></div>';
        });
        html += '</div></div>';
        return html;
    }

    // --- Page viewer ---
    function renderPageView(el) {
        el.innerHTML = '<div class="page-viewer"><p style="color:var(--text-dim)">Loading…</p></div>';
        loadPage(state.pagePath).then(function (page) {
            var html = '<div class="page-viewer">';
            html += '<h1>' + esc(page.path.split('/').pop().replace(/\.txt$/, '')) + '</h1>';
            html += '<div class="page-path">' + esc(page.path) + '</div>';
            html += renderBreadcrumb(page.path.split('/').slice(0, -1).join('/'));
            html += '<div class="page-content">' + renderMarkdown(page.content) + '</div>';
            html += '</div>';
            el.innerHTML = html;
        }).catch(function () {
            el.innerHTML = render404();
        });
    }

    // --- Simple markdown renderer ---
    function renderMarkdown(text) {
        var lines = text.split('\n');
        var html = '';
        var inList = false;
        for (var i = 0; i < lines.length; i++) {
            var line = lines[i];
            // Headings
            if (line.match(/^### /)) { html += closeList(); html += '<h3>' + esc(line.slice(4)) + '</h3>'; continue; }
            if (line.match(/^## /)) { html += closeList(); html += '<h2>' + esc(line.slice(3)) + '</h2>'; continue; }
            if (line.match(/^# /)) { html += closeList(); html += '<h1>' + esc(line.slice(2)) + '</h1>'; continue; }
            // List items
            if (line.match(/^- /)) {
                if (!inList) { html += '<ul>'; inList = true; }
                html += '<li>' + inlineFormat(line.slice(2)) + '</li>';
                continue;
            }
            // Empty line
            if (line.trim() === '') { html += closeList(); html += '<br>'; continue; }
            // Regular text
            html += closeList();
            html += '<p>' + inlineFormat(line) + '</p>';
        }
        html += closeList();
        return html;

        function closeList() {
            if (inList) { inList = false; return '</ul>'; }
            return '';
        }
    }

    function inlineFormat(text) {
        // Bold
        text = esc(text);
        text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
        // Inline code
        text = text.replace(/`(.+?)`/g, '<code>$1</code>');
        // Links
        text = text.replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2">$1</a>');
        return text;
    }

    // --- 404 ---
    function render404() {
        return '<div class="not-found"><h1>404</h1><p>Page not found</p><a href="#/">← Back to dashboard</a></div>';
    }

    // --- Activity panel ---
    function renderActivityPanel() {
        var panel = document.getElementById('activity-panel');
        panel.classList.toggle('collapsed', !state.activityOpen);
        var list = document.getElementById('activity-list');
        if (!state.activityOpen) return;

        if (state.activity.length === 0) {
            list.innerHTML = '<div class="activity-empty">No activity yet</div>';
            return;
        }
        var html = '';
        state.activity.forEach(function (a) {
            html += '<div class="activity-item">';
            html += '<div class="activity-item-kind ' + esc(a.kind) + '">' + esc(a.kind) + '</div>';
            html += '<div class="activity-item-path">' + esc(a.path) + '</div>';
            html += '<div class="activity-item-time">' + esc(a.time) + '</div>';
            html += '</div>';
        });
        list.innerHTML = html;
    }

    // --- Helpers ---
    function findFolder(entries, name) {
        for (var i = 0; i < (entries || []).length; i++) {
            if (entries[i].isDir && entries[i].name === name) return entries[i];
        }
        return null;
    }

    function resolveEntries(tree, path) {
        if (!path) return tree;
        var parts = path.split('/');
        var current = tree;
        for (var i = 0; i < parts.length; i++) {
            var found = null;
            for (var j = 0; j < (current || []).length; j++) {
                if (current[j].name === parts[i]) { found = current[j]; break; }
            }
            if (!found) return null;
            current = found.children || [];
        }
        return current;
    }

    function findConcernForPath(path) {
        if (!path) return null;
        var topFolder = path.split('/')[0];
        var concerns = state.config.concerns || [];
        for (var i = 0; i < concerns.length; i++) {
            if (concerns[i].folderName === topFolder) return concerns[i];
        }
        return null;
    }

    function countFiles(entries) {
        var count = 0;
        (entries || []).forEach(function (e) {
            if (e.isDir) count += countFiles(e.children);
            else count++;
        });
        return count;
    }

    function esc(s) {
        if (!s) return '';
        var d = document.createElement('div');
        d.textContent = s;
        return d.innerHTML;
    }

    // --- Search ---
    var searchTimeout;
    function setupSearch() {
        var input = document.getElementById('search-input');
        var dropdown = document.getElementById('search-results');
        input.addEventListener('input', function () {
            clearTimeout(searchTimeout);
            var q = input.value.trim();
            if (q.length < 2) { dropdown.hidden = true; return; }
            searchTimeout = setTimeout(function () {
                loadSearch(q).then(function (results) {
                    state.searchResults = results;
                    if (results.length === 0) {
                        dropdown.innerHTML = '<div class="search-empty">No results</div>';
                    } else {
                        var html = '';
                        results.slice(0, 20).forEach(function (r) {
                            html += '<div class="search-hit" data-path="' + esc(r.path) + '">';
                            html += '<div class="search-hit-path">' + esc(r.path) + '</div>';
                            html += '<div class="search-hit-line">line ' + r.line + '</div>';
                            html += '<div class="search-hit-ctx">' + esc(r.context) + '</div>';
                            html += '</div>';
                        });
                        dropdown.innerHTML = html;
                    }
                    dropdown.hidden = false;
                });
            }, 200);
        });
        dropdown.addEventListener('click', function (e) {
            var hit = e.target.closest('.search-hit');
            if (hit) {
                location.hash = '#/explore/' + hit.dataset.path;
                dropdown.hidden = true;
                input.value = '';
            }
        });
        document.addEventListener('click', function (e) {
            if (!e.target.closest('.search-box')) dropdown.hidden = true;
        });
    }

    // --- Init ---
    function init() {
        setupSearch();

        document.getElementById('activity-toggle').addEventListener('click', function () {
            state.activityOpen = !state.activityOpen;
            renderActivityPanel();
        });
        document.getElementById('activity-close').addEventListener('click', function () {
            state.activityOpen = false;
            renderActivityPanel();
        });

        window.addEventListener('hashchange', navigate);

        // Initial load
        Promise.all([loadConfig(), loadPages(), loadActivity()]).then(function (results) {
            state.config = results[0];
            state.pages = results[1];
            state.activity = results[2];
            navigate();
        }).catch(function () {
            navigate();
        });

        connectSSE();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
