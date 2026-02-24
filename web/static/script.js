document.addEventListener('DOMContentLoaded', () => {
    const app = document.getElementById('app');
    const chatContainer = document.getElementById('chat-container');
    const messageInput = document.getElementById('message-input');
    const sendBtn = document.getElementById('send-btn');
    const conversationList = document.getElementById('conversation-list');
    const newChatBtn = document.getElementById('new-chat-btn');
    const sidebarToggle = document.getElementById('sidebar-toggle');
    const settingsBtn = document.getElementById('settings-btn');
    const settingsModal = document.getElementById('settings-modal');
    const closeSettings = document.getElementById('close-settings');
    const saveKBFolder = document.getElementById('save-kb-folder');
    const selectKBFolder = document.getElementById('select-kb-folder');
    const kbFolderInput = document.getElementById('kb-folder-input');
    const syncKBBtn = document.getElementById('sync-kb-btn');
    const syncStatus = document.getElementById('sync-status');
    const kbFileList = document.getElementById('kb-file-list');
    const currentConversationTitle = document.getElementById('current-conversation-title');

    let currentConversationId = null;
    let conversations = [];

    function escapeHtml(str) {
        return String(str)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function isSafeUrl(url) {
        try {
            const u = new URL(url, window.location.origin);
            return u.protocol === 'http:' || u.protocol === 'https:';
        } catch {
            return false;
        }
    }

    function renderInlineMarkdown(text) {
        let s = escapeHtml(text);
        s = s.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
        s = s.replace(/`([^`]+?)`/g, '<code>$1</code>');
        s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, label, url) => {
            if (!isSafeUrl(url)) return label;
            return `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer">${label}</a>`;
        });
        return s;
    }

    function renderMarkdown(text) {
        const parts = String(text || '').split('```');
        let html = '';
        for (let i = 0; i < parts.length; i++) {
            if (i % 2 === 1) {
                const block = parts[i];
                const lines = block.split('\n');
                const first = lines[0].trim();
                const rest = lines.slice(1).join('\n').trimEnd();
                const lang = first || '';
                const langAttr = lang ? ` class="language-${lang}" data-lang="${escapeHtml(lang)}"` : '';
                const label = lang || '代码';
                html += `<div class="code-block"><div class="code-block-header"><span class="code-lang">${escapeHtml(label)}</span><button class="copy-btn" type="button">复制</button></div><pre><code${langAttr}>${escapeHtml(rest)}</code></pre></div>`;
                continue;
            }

            const lines = parts[i].split('\n');
            let inList = false;
            for (const rawLine of lines) {
                const line = rawLine.replace(/\r/g, '');
                const m = /^(\s*[-*])\s+(.*)$/.exec(line);
                if (m) {
                    if (!inList) {
                        inList = true;
                        html += '<ul>';
                    }
                    html += `<li>${renderInlineMarkdown(m[2])}</li>`;
                } else {
                    if (inList) {
                        html += '</ul>';
                        inList = false;
                    }
                    if (line.trim() === '') {
                        html += '<br />';
                    } else {
                        html += `<p>${renderInlineMarkdown(line)}</p>`;
                    }
                }
            }
            if (inList) html += '</ul>';
        }
        return html;
    }

    function appendMessage(role, content) {
        const messageDiv = document.createElement('div');
        messageDiv.classList.add('message', role);
        if (role === 'assistant') {
            messageDiv.innerHTML = renderMarkdown(content);
        } else {
            messageDiv.textContent = content;
        }
        chatContainer.appendChild(messageDiv);
        chatContainer.scrollTop = chatContainer.scrollHeight;
        return messageDiv;
    }

    function formatRelativeTime(iso) {
        const t = new Date(iso).getTime();
        if (!t) return '';
        const diff = Date.now() - t;
        const min = Math.floor(diff / 60000);
        if (min < 1) return '刚刚';
        if (min < 60) return `${min} 分钟前`;
        const hr = Math.floor(min / 60);
        if (hr < 24) return `${hr} 小时前`;
        const day = Math.floor(hr / 24);
        return `${day} 天前`;
    }

    function renderConversationList() {
        conversationList.innerHTML = '';
        conversations.forEach(c => {
            const item = document.createElement('div');
            item.className = 'conversation-item' + (String(c.ID) === String(currentConversationId) ? ' active' : '');
            item.dataset.id = String(c.ID);

            const title = document.createElement('div');
            title.className = 'conversation-item-title';
            title.textContent = c.Title || `Chat ${c.ID}`;

            const meta = document.createElement('div');
            meta.className = 'conversation-item-meta';
            meta.textContent = formatRelativeTime(c.UpdatedAt || c.CreatedAt);

            const actions = document.createElement('div');
            actions.className = 'conversation-item-actions';

            const deleteBtn = document.createElement('button');
            deleteBtn.type = 'button';
            deleteBtn.className = 'conversation-delete-btn';
            deleteBtn.textContent = '删除';
            deleteBtn.addEventListener('click', async (e) => {
                e.stopPropagation();
                const id = Number(item.dataset.id);
                if (!id) return;
                if (!window.confirm('确定要删除这个会话吗？')) return;
                try {
                    const res = await fetch(`/api/conversations/${id}`, { method: 'DELETE' });
                    if (!res.ok) {
                        const t = await res.text();
                        throw new Error(t || 'Failed to delete conversation');
                    }
                    conversations = conversations.filter(x => String(x.ID) !== String(id));
                    if (String(currentConversationId) === String(id)) {
                        const first = conversations && conversations.length ? conversations[0].ID : null;
                        if (first) {
                            await switchConversation(first);
                        } else {
                            currentConversationId = null;
                            localStorage.removeItem('conversationId');
                            currentConversationTitle.textContent = '离线本地知识库';
                            chatContainer.innerHTML = '';
                            renderConversationList();
                        }
                    } else {
                        renderConversationList();
                    }
                } catch (err) {
                    console.error(err);
                    alert('删除失败');
                }
            });

            actions.appendChild(deleteBtn);

            item.appendChild(title);
            item.appendChild(meta);
            item.appendChild(actions);

            item.addEventListener('click', async () => {
                const id = Number(item.dataset.id);
                if (!id) return;
                await switchConversation(id);
                if (app.classList.contains('sidebar-open')) {
                    app.classList.remove('sidebar-open');
                }
            });

            conversationList.appendChild(item);
        });
    }

    function setCurrentConversation(id) {
        currentConversationId = id;
        localStorage.setItem('conversationId', String(id));
        const c = conversations.find(x => String(x.ID) === String(id));
        currentConversationTitle.textContent = c ? (c.Title || `Chat ${c.ID}`) : '离线本地知识库';
        renderConversationList();
    }

    async function loadMessages(conversationId) {
        chatContainer.innerHTML = '';
        const res = await fetch(`/api/conversations/${conversationId}/messages`);
        if (!res.ok) throw new Error('Failed to load messages');
        const data = await res.json();
        if (data) {
            const lastUser = [...data].reverse().find(m => m.Role === 'user');
            const lastAssistant = [...data].reverse().find(m => m.Role === 'assistant');
            data.forEach(msg => {
                const canEdit = lastUser && msg.Role === 'user' && msg.ID === lastUser.ID;
                const canRetry = lastUser && lastAssistant && msg.Role === 'assistant' && msg.ID === lastAssistant.ID;
                const el = appendMessage(msg.Role, msg.Content);
                el.dataset.id = String(msg.ID || '');
                if (canEdit || canRetry) {
                    el.classList.add('has-actions');
                    const actions = document.createElement('div');
                    actions.className = 'message-actions';
                    if (canEdit) {
                        const btn = document.createElement('button');
                        btn.type = 'button';
                        btn.className = 'msg-btn';
                        btn.textContent = '编辑';
                        btn.addEventListener('click', (e) => {
                            e.stopPropagation();
                            startEditMessage(el, msg);
                        });
                        actions.appendChild(btn);
                    }
                    if (canRetry) {
                        const btn = document.createElement('button');
                        btn.type = 'button';
                        btn.className = 'msg-btn';
                        btn.textContent = '重试';
                        btn.addEventListener('click', async (e) => {
                            e.stopPropagation();
                            el.remove();
                            await retryStream();
                        });
                        actions.appendChild(btn);
                    }
                    el.appendChild(actions);
                }
            });
        }
    }

    async function refreshConversations() {
        const res = await fetch('/api/conversations');
        if (!res.ok) return;
        conversations = await res.json();
        const c = conversations.find(x => String(x.ID) === String(currentConversationId));
        currentConversationTitle.textContent = c ? (c.Title || `Chat ${c.ID}`) : '离线本地知识库';
        renderConversationList();
    }

    async function startEditMessage(messageEl, msg) {
        const original = msg.Content || '';
        messageEl.innerHTML = '';

        const ta = document.createElement('textarea');
        ta.className = 'edit-textarea';
        ta.value = original;
        messageEl.appendChild(ta);

        const row = document.createElement('div');
        row.className = 'edit-actions';

        const cancelBtn = document.createElement('button');
        cancelBtn.type = 'button';
        cancelBtn.className = 'msg-btn';
        cancelBtn.textContent = '取消';
        cancelBtn.addEventListener('click', async () => {
            await loadMessages(currentConversationId);
        });

        const saveBtn = document.createElement('button');
        saveBtn.type = 'button';
        saveBtn.className = 'msg-btn primary';
        saveBtn.textContent = '保存';
        saveBtn.addEventListener('click', async () => {
            const content = ta.value.trim();
            if (!content) return;
            try {
                const res = await fetch(`/api/conversations/${currentConversationId}/messages/${msg.ID}`, {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content })
                });
                if (!res.ok) {
                    const t = await res.text();
                    throw new Error(t || 'Failed to edit message');
                }
                await loadMessages(currentConversationId);
                await retryStream();
            } catch (e) {
                console.error(e);
                await loadMessages(currentConversationId);
            }
        });

        row.appendChild(cancelBtn);
        row.appendChild(saveBtn);
        messageEl.appendChild(row);

        ta.focus();
        ta.setSelectionRange(ta.value.length, ta.value.length);
    }

    async function retryStream() {
        if (!currentConversationId) return;
        const assistantDiv = appendMessage('assistant', '');
        sendBtn.disabled = true;

        try {
            const response = await fetch(`/api/conversations/${currentConversationId}/retry/stream`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' }
            });
            if (!response.ok) {
                const t = await response.text();
                throw new Error(t || 'Network response was not ok');
            }

            const reader = response.body.getReader();
            const decoder = new TextDecoder('utf-8');
            let raw = '';
            while (true) {
                const { value, done } = await reader.read();
                if (done) break;
                const chunk = decoder.decode(value, { stream: true });
                if (chunk) {
                    raw += chunk;
                    assistantDiv.textContent = raw;
                    chatContainer.scrollTop = chatContainer.scrollHeight;
                }
            }
            assistantDiv.innerHTML = renderMarkdown(raw);
            await refreshConversations();
        } catch (error) {
            console.error('Error:', error);
            assistantDiv.textContent = 'Error: ' + error.message;
        } finally {
            sendBtn.disabled = false;
            messageInput.focus();
        }
    }

    async function switchConversation(id) {
        setCurrentConversation(id);
        await loadMessages(id);
    }

    async function loadConversations() {
        const res = await fetch('/api/conversations');
        if (!res.ok) throw new Error('Failed to load conversations');
        conversations = await res.json();

        const saved = localStorage.getItem('conversationId');
        const savedId = saved ? Number(saved) : null;
        const firstId = conversations && conversations.length ? conversations[0].ID : null;
        const pickedId = (savedId && conversations.some(c => c.ID === savedId)) ? savedId : firstId;
        if (pickedId) {
            await switchConversation(pickedId);
        } else {
            renderConversationList();
        }
    }

    async function createConversation() {
        const res = await fetch('/api/conversations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title: 'New chat' })
        });
        if (!res.ok) throw new Error('Failed to create conversation');
        const conv = await res.json();
        await loadConversations();
        await switchConversation(conv.ID);
    }

    async function sendMessage() {
        const message = messageInput.value.trim();
        if (!message) return;
        if (!currentConversationId) return;

        appendMessage('user', message);
        const assistantDiv = appendMessage('assistant', '');
        messageInput.value = '';
        sendBtn.disabled = true;

        try {
            const response = await fetch(`/api/conversations/${currentConversationId}/chat/stream`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ message: message })
            });

            if (!response.ok) {
                const t = await response.text();
                throw new Error(t || 'Network response was not ok');
            }

            const reader = response.body.getReader();
            const decoder = new TextDecoder('utf-8');
            let raw = '';
            while (true) {
                const { value, done } = await reader.read();
                if (done) break;
                const chunk = decoder.decode(value, { stream: true });
                if (chunk) {
                    raw += chunk;
                    assistantDiv.textContent = raw;
                    chatContainer.scrollTop = chatContainer.scrollHeight;
                }
            }
            assistantDiv.innerHTML = renderMarkdown(raw);
            await refreshConversations();
        } catch (error) {
            console.error('Error:', error);
            assistantDiv.textContent = 'Error: ' + error.message;
        } finally {
            sendBtn.disabled = false;
            messageInput.focus();
        }
    }

    sendBtn.addEventListener('click', sendMessage);
    newChatBtn.addEventListener('click', createConversation);

    sidebarToggle.addEventListener('click', () => {
        app.classList.toggle('sidebar-open');
    });

    chatContainer.addEventListener('click', async (e) => {
        const el = e.target;
        if (!(el instanceof HTMLElement)) return;
        if (!el.classList.contains('copy-btn')) return;
        const wrapper = el.closest('.code-block');
        if (!wrapper) return;
        const code = wrapper.querySelector('pre code');
        if (!code) return;
        try {
            await navigator.clipboard.writeText(code.textContent || '');
            el.textContent = '已复制';
            setTimeout(() => {
                el.textContent = '复制';
            }, 1200);
        } catch (err) {
            console.error(err);
        }
    });

    messageInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            sendMessage();
        }
    });

    // 设置相关逻辑
    settingsBtn.addEventListener('click', () => {
        settingsModal.style.display = 'block';
        loadSettings();
        loadKBFiles();
    });

    closeSettings.addEventListener('click', () => {
        settingsModal.style.display = 'none';
    });

    window.addEventListener('click', (e) => {
        if (e.target === settingsModal) {
            settingsModal.style.display = 'none';
        }
    });

    async function loadSettings() {
        try {
            const res = await fetch('/api/settings/kb-folder');
            const data = await res.json();
            if (data.folder) {
                kbFolderInput.value = data.folder;
            }
        } catch (err) {
            console.error('Failed to load settings:', err);
        }
    }

    saveKBFolder.addEventListener('click', async () => {
        const folder = kbFolderInput.value.trim();
        try {
            const res = await fetch('/api/settings/kb-folder', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ value: folder })
            });
            if (res.ok) {
                alert('保存成功');
            } else {
                alert('保存失败');
            }
        } catch (err) {
            console.error('Failed to save settings:', err);
            alert('保存出错');
        }
    });

    selectKBFolder.addEventListener('click', async () => {
        try {
            const res = await fetch('/api/settings/select-folder', {
                method: 'POST'
            });
            const data = await res.json();
            if (data.path) {
                kbFolderInput.value = data.path;
            }
        } catch (err) {
            console.error('Failed to select folder:', err);
        }
    });

    syncKBBtn.addEventListener('click', async () => {
        syncStatus.textContent = '正在同步...';
        syncKBBtn.disabled = true;
        try {
            const res = await fetch('/api/kb/sync', { method: 'POST' });
            if (res.ok) {
                syncStatus.textContent = '同步已开始';
                // 每隔几秒刷新一下文件列表
                const timer = setInterval(async () => {
                    const finished = await loadKBFiles();
                    if (finished) {
                        clearInterval(timer);
                        syncStatus.textContent = '同步完成';
                        syncKBBtn.disabled = false;
                    }
                }, 2000);
            } else {
                syncStatus.textContent = '同步失败';
                syncKBBtn.disabled = false;
            }
        } catch (err) {
            console.error('Failed to sync KB:', err);
            syncStatus.textContent = '同步出错';
            syncKBBtn.disabled = false;
        }
    });

    async function loadKBFiles() {
        try {
            const res = await fetch('/api/kb/files');
            const files = await res.json();
            kbFileList.innerHTML = '';
            
            let allProcessed = true;
            if (files.length === 0) {
                kbFileList.innerHTML = '<div class="file-item">无文件</div>';
                return true;
            }

            files.forEach(f => {
                const item = document.createElement('div');
                item.className = 'file-item';
                
                const fileName = f.Path.split('/').pop();
                const statusClass = `status-${f.Status}`;
                const statusText = f.Status === 'processed' ? '已处理' : 
                                  f.Status === 'pending' ? '处理中' : '错误';
                
                if (f.Status !== 'processed') allProcessed = false;

                item.innerHTML = `
                    <span title="${f.Path}">${fileName}</span>
                    <span class="file-status ${statusClass}">${statusText}</span>
                `;
                kbFileList.appendChild(item);
            });
            return allProcessed;
        } catch (err) {
            console.error('Failed to load KB files:', err);
            return true;
        }
    }

    // 初始化
    loadConversations().catch(err => console.error('Failed to load conversations:', err));
});
