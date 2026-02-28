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
    const modelSelect = document.getElementById('model-select');
    const closeSettings = document.getElementById('close-settings');
    const saveKBFolder = document.getElementById('save-kb-folder');
    const selectKBFolder = document.getElementById('select-kb-folder');
    const kbFolderInput = document.getElementById('kb-folder-input');
    const syncKBBtn = document.getElementById('sync-kb-btn');
    const resetKBBtn = document.getElementById('reset-kb-btn');
    const syncStatus = document.getElementById('sync-status');
    const syncProgressContainer = document.getElementById('sync-progress-container');
    const syncProgressBar = document.getElementById('sync-progress-bar');
    const syncProgressText = document.getElementById('sync-progress-text');
    const syncCurrentFile = document.getElementById('sync-current-file');
    const chunkProgressContainer = document.getElementById('chunk-progress-container');
    const chunkProgressContent = document.getElementById('chunk-progress-content');
    const kbFileList = document.getElementById('kb-file-list');
    const currentConversationTitle = document.getElementById('current-conversation-title');
    const uploadKBBtn = document.getElementById('upload-kb-btn');
    const kbFileUpload = document.getElementById('kb-file-upload');
    const chatFileInput = document.getElementById('chat-file-input');
    const attachBtn = document.getElementById('attach-btn');
    const filePreviewContainer = document.getElementById('file-preview-container');

    // 预览模态框元素
    const previewModal = document.getElementById('preview-modal');
    const previewTitle = document.getElementById('preview-title');
    const previewBody = document.getElementById('preview-body');
    const closePreviewBtn = document.getElementById('close-preview');

    // Batch Operations Elements
    const manageChatsBtn = document.getElementById('manage-chats-btn');
    const batchActions = document.getElementById('batch-actions');
    const selectAllChatsBtn = document.getElementById('select-all-chats-btn');
    const deleteSelectedChatsBtn = document.getElementById('delete-selected-chats-btn');
    const cancelManageBtn = document.getElementById('cancel-manage-btn');
    const deleteSelectedFilesBtn = document.getElementById('delete-selected-files-btn');
    
    const LOADING_HTML = '<div class="loading-dots"><div class="loading-dot"></div><div class="loading-dot"></div><div class="loading-dot"></div></div>';

    let currentConversationId = null;
    let conversations = [];
    
    // Batch State
    let isManagementMode = false;
    let selectedConversationIds = new Set();
    let selectedFileIds = new Set();

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

    function normalizeMarkdownTableChars(s) {
        // 兼容中文输入法/模型输出的全角符号
        return String(s ?? '')
            .replaceAll('｜', '|')   // fullwidth vertical bar
            .replaceAll('—', '-')   // em dash
            .replaceAll('–', '-')   // en dash
            .replaceAll('－', '-')  // fullwidth hyphen-minus
            .replaceAll('‑', '-');  // non-breaking hyphen
    }

    function renderParagraphsAndLists(lines) {
        if (!lines || lines.length === 0) return '';

        let html = '';
        let inList = false;

        for (const rawLine of lines) {
            const line = String(rawLine ?? '').replace(/\r/g, '');
            const m = /^(\s*[-*])\s+(.*)$/.exec(line);

            if (m) {
                if (!inList) {
                    inList = true;
                    html += '<ul>';
                }
                html += `<li>${renderInlineMarkdown(m[2])}</li>`;
                continue;
            }

            if (inList) {
                html += '</ul>';
                inList = false;
            }

            if (line.trim() === '') {
                continue;
            }
            html += `<p>${renderInlineMarkdown(line)}</p>`;
        }

        if (inList) html += '</ul>';
        return html;
    }

    function splitMarkdownTableRowFromFirstPipe(rawLine) {
        const s = normalizeMarkdownTableChars(rawLine);
        const pipeIndex = s.indexOf('|');
        if (pipeIndex === -1) return null;

        const row = s.slice(pipeIndex);
        const parts = row.split('|').map(x => x.trim());

        // 移除边界 '|' 产生的空项，但保留中间的空单元格（如 "a||b"）
        if (parts.length && parts[0] === '') parts.shift();
        if (parts.length && parts[parts.length - 1] === '') parts.pop();
        return parts;
    }

    function parseAlignmentCell(cell) {
        const c = normalizeMarkdownTableChars(cell).trim().replace(/\s+/g, '');
        if (!c) return null;
        // markdown 标准通常是 3+ 个 '-'，但这里放宽为 1+，以兼容模型输出
        if (!/^:?-+:?$/.test(c)) return null;
        const left = c.startsWith(':');
        const right = c.endsWith(':');
        if (left && right) return 'center';
        if (right) return 'right';
        return 'left';
    }

    function renderMarkdownTableHtml(headers, alignments, rows) {
        const colCount = headers.length;
        let html = '<div class="markdown-table-container"><table class="markdown-table">';

        html += '<thead><tr>';
        for (let i = 0; i < colCount; i++) {
            const align = alignments[i] || 'left';
            html += `<th style="text-align: ${align}">${renderInlineMarkdown(headers[i] ?? '')}</th>`;
        }
        html += '</tr></thead>';

        html += '<tbody>';
        for (const row of rows) {
            html += '<tr>';
            for (let i = 0; i < colCount; i++) {
                const align = alignments[i] || 'left';
                html += `<td style="text-align: ${align}">${renderInlineMarkdown(row[i] ?? '')}</td>`;
            }
            html += '</tr>';
        }
        html += '</tbody></table></div>';

        return html;
    }

    function tryParseTableAt(lines, startIndex) {
        if (!Array.isArray(lines)) return null;
        if (startIndex < 0 || startIndex + 1 >= lines.length) return null;

        const headerRaw = normalizeMarkdownTableChars(lines[startIndex]);
        const separatorRaw = normalizeMarkdownTableChars(lines[startIndex + 1]);

        const headerPipeIndex = headerRaw.indexOf('|');
        const separatorPipeIndex = separatorRaw.indexOf('|');
        if (headerPipeIndex === -1 || separatorPipeIndex === -1) return null;

        const prefix = headerRaw.slice(0, headerPipeIndex);
        const separatorPart = separatorRaw.slice(separatorPipeIndex).trim();
        if (!/^[|\-:\s]+$/.test(separatorPart)) return null;

        const headers = splitMarkdownTableRowFromFirstPipe(headerRaw);
        if (!headers || headers.length < 2) return null;

        const separatorCells = splitMarkdownTableRowFromFirstPipe(separatorRaw);
        if (!separatorCells || separatorCells.length < headers.length) return null;

        const alignments = [];
        for (let i = 0; i < headers.length; i++) {
            const a = parseAlignmentCell(separatorCells[i]);
            if (!a) return null; // 分隔线不合法就不当表格
            alignments.push(a);
        }

        const rows = [];
        let i = startIndex + 2;
        while (i < lines.length) {
            const raw = normalizeMarkdownTableChars(lines[i]);
            if (raw.trim() === '') break;
            if (!raw.includes('|')) break;

            const cells = splitMarkdownTableRowFromFirstPipe(raw);
            if (!cells || cells.length === 0) break;

            const normalized = cells.slice(0, headers.length);
            while (normalized.length < headers.length) normalized.push('');
            rows.push(normalized);
            i++;
        }

        return {
            prefix: prefix.trimEnd(),
            headers,
            alignments,
            rows,
            nextIndex: i
        };
    }

    function renderTextWithTables(text) {
        const lines = normalizeMarkdownTableChars(text).split('\n');
        let html = '';
        let buffer = [];

        for (let i = 0; i < lines.length; ) {
            const parsed = tryParseTableAt(lines, i);
            if (!parsed) {
                buffer.push(lines[i]);
                i++;
                continue;
            }

            // 表头行前的前缀文字（如“我的对话中返回的 ”）要保留为普通文本
            if (parsed.prefix && parsed.prefix.trim() !== '') {
                buffer.push(parsed.prefix);
            }

            html += renderParagraphsAndLists(buffer);
            buffer = [];

            html += renderMarkdownTableHtml(parsed.headers, parsed.alignments, parsed.rows);
            i = parsed.nextIndex;
        }

        html += renderParagraphsAndLists(buffer);
        return html;
    }

    function renderTable(text) {
        const lines = normalizeMarkdownTableChars(text).trim().split('\n');
        if (lines.length < 2) return '';

        for (let i = 0; i < lines.length - 1; i++) {
            const parsed = tryParseTableAt(lines, i);
            if (!parsed) continue;
            return renderMarkdownTableHtml(parsed.headers, parsed.alignments, parsed.rows);
        }
        return '';
    }

    function renderMarkdown(text) {
        if (!text) return '';

        // 匹配代码块、think 块、document 块、knowledge_base 块
        // 支持未闭合的标签以实现渐进式渲染
        const regex = /(```[\s\S]*?(?:```|$)|<think>[\s\S]*?(?:<\/think>|$)|<document\s+index="\d+">[\s\S]*?<\/document>|<knowledge_base>[\s\S]*?(?:<\/knowledge_base>|$))/gi;
        const parts = String(text).split(regex);
        let html = '';
        
        for (const part of parts) {
            if (!part) continue;
            
            // 处理代码块
            if (part.startsWith('```')) {
                let block = part.substring(3);
                if (block.endsWith('```')) {
                    block = block.substring(0, block.length - 3);
                }
                
                const lines = block.split('\n');
                const first = lines[0].trim();
                const rest = lines.slice(1).join('\n').trimEnd();
                
                const lang = first || '';
                const langAttr = lang ? ` class="language-${lang}" data-lang="${escapeHtml(lang)}"` : '';
                const label = lang || '代码';
                html += `<div class="code-block"><div class="code-block-header"><span class="code-lang">${escapeHtml(label)}</span><button class="copy-btn" type="button">复制</button></div><pre><code${langAttr}>${escapeHtml(rest)}</code></pre></div>`;
                continue;
            }
            
            // 处理 <think> 块
            if (/^<think>/i.test(part)) {
                let content = part.replace(/^<think>/i, '');
                let isClosed = false;
                if (/<\/think>$/i.test(content)) {
                    content = content.replace(/<\/think>$/i, '');
                    isClosed = true;
                }
                
                // 检查内容是否为空
                const trimmedContent = content.trim();
                if (trimmedContent === '') {
                    continue; // 跳过空的思考过程块
                }
                
                // 默认展开，不添加 collapsed 类
                // 如果未闭合，在内容末尾添加 loading 动画
                let innerHtml = renderMarkdown(content);
                if (!isClosed) {
                    innerHtml += '<div class="loading-dots"><div class="loading-dot"></div><div class="loading-dot"></div><div class="loading-dot"></div></div>';
                }
                
                html += `<div class="think-block"><div class="think-title">思考过程</div><div class="think-content">${innerHtml}</div></div>`;
                continue;
            }
            
            // 处理 <document> 块
            const docMatch = /^<document\s+index="(\d+)">/i.exec(part);
            if (docMatch) {
                const index = docMatch[1];
                const content = part.replace(/^<document\s+index="\d+">/i, '').replace(/<\/document>$/i, '');
                html += `<div class="document-block"><div class="document-title">文档引用 #${index}</div><div class="document-content">${renderMarkdown(content)}</div></div>`;
                continue;
            }
            
            // 处理 <knowledge_base> 块
            if (/^<knowledge_base>/i.test(part)) {
                const content = part.replace(/^<knowledge_base>/i, '').replace(/<\/knowledge_base>$/i, '');
                html += `<div class="knowledge-base-block"><div class="knowledge-base-title">知识库引用</div><div class="knowledge-base-content">${renderMarkdown(content)}</div></div>`;
                continue;
            }

            // 普通文本 + 表格混排渲染
            html += renderTextWithTables(part);
        }
        return html;
    }

    function updateMessageContent(element, markdown) {
        // 1. Capture state of existing think blocks
        const existingThinkBlocks = element.querySelectorAll('.think-block');
        const states = Array.from(existingThinkBlocks).map(block => !block.classList.contains('collapsed'));
        
        // 2. Render new HTML
        const newHtml = renderMarkdown(markdown);
        
        // 3. Apply
        element.innerHTML = newHtml;
        
        // 4. Restore state
        const newThinkBlocks = element.querySelectorAll('.think-block');
        newThinkBlocks.forEach((block, index) => {
            if (states[index]) { // if it was expanded
                block.classList.remove('collapsed');
            }
        });
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

            // Batch Selection Checkbox (Always render, hidden by CSS)
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.className = 'conversation-checkbox';
            checkbox.checked = selectedConversationIds.has(c.ID);
            checkbox.addEventListener('click', (e) => {
                e.stopPropagation();
                if (checkbox.checked) {
                    selectedConversationIds.add(c.ID);
                } else {
                    selectedConversationIds.delete(c.ID);
                }
                updateBatchDeleteButtonState();
            });
            item.appendChild(checkbox);

            // Content Wrapper
            const content = document.createElement('div');
            content.className = 'conversation-content';

            const title = document.createElement('div');
            title.className = 'conversation-item-title';
            title.textContent = c.Title || `Chat ${c.ID}`;

            const meta = document.createElement('div');
            meta.className = 'conversation-item-meta';
            meta.textContent = formatRelativeTime(c.UpdatedAt || c.CreatedAt);

            content.appendChild(title);
            content.appendChild(meta);
            item.appendChild(content);

            const actions = document.createElement('div');
            actions.className = 'conversation-item-actions';

            // Delete Button (Always render, hidden by CSS in batch mode)
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
            item.appendChild(actions);

            item.addEventListener('click', async () => {
                if (isManagementMode) {
                    // Toggle selection on item click
                    const id = c.ID;
                    if (selectedConversationIds.has(id)) {
                        selectedConversationIds.delete(id);
                        checkbox.checked = false;
                    } else {
                        selectedConversationIds.add(id);
                        checkbox.checked = true;
                    }
                    updateBatchDeleteButtonState();
                    return;
                }
                
                const id = Number(item.dataset.id);
                if (!id) return;
                await switchConversation(id);
                if (app.classList.contains('sidebar-open')) {
                    app.classList.remove('sidebar-open');
                }
            });

            conversationList.appendChild(item);
        });
        
        updateBatchDeleteButtonState();
    }

    function updateBatchDeleteButtonState() {
        if (selectedConversationIds.size > 0) {
            deleteSelectedChatsBtn.textContent = `删除选中 (${selectedConversationIds.size})`;
            deleteSelectedChatsBtn.disabled = false;
        } else {
            deleteSelectedChatsBtn.textContent = '删除选中';
            deleteSelectedChatsBtn.disabled = true;
        }
    }

    function toggleManagementMode(enabled) {
        isManagementMode = enabled;
        selectedConversationIds.clear();
        
        const sidebar = document.querySelector('.sidebar');
        if (enabled) {
            sidebar.classList.add('batch-mode');
            batchActions.classList.add('active');
            sidebarToggle.style.display = 'none'; 
        } else {
            sidebar.classList.remove('batch-mode');
            batchActions.classList.remove('active');
            sidebarToggle.style.display = '';
        }
        
        // Reset checkboxes
        document.querySelectorAll('.conversation-checkbox').forEach(cb => cb.checked = false);
        updateBatchDeleteButtonState();
    }

    // Batch Action Event Listeners
    manageChatsBtn.addEventListener('click', () => {
        toggleManagementMode(true);
    });

    cancelManageBtn.addEventListener('click', () => {
        toggleManagementMode(false);
    });

    selectAllChatsBtn.addEventListener('click', () => {
        const allCheckboxes = document.querySelectorAll('.conversation-checkbox');
        if (selectedConversationIds.size === conversations.length) {
            selectedConversationIds.clear();
            allCheckboxes.forEach(cb => cb.checked = false);
        } else {
            conversations.forEach(c => selectedConversationIds.add(c.ID));
            allCheckboxes.forEach(cb => cb.checked = true);
        }
        updateBatchDeleteButtonState();
    });

    deleteSelectedChatsBtn.addEventListener('click', async () => {
        if (selectedConversationIds.size === 0) return;
        if (!confirm(`确定要删除选中的 ${selectedConversationIds.size} 个会话吗？`)) return;

        try {
            const ids = Array.from(selectedConversationIds);
            const res = await fetch('/api/conversations/batch-delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ids })
            });
            
            if (!res.ok) {
                const t = await res.text();
                throw new Error(t);
            }

            // Refresh
            await loadConversations();
            toggleManagementMode(false);
        } catch (err) {
            console.error(err);
            alert('批量删除失败: ' + err.message);
        }
    });

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
        assistantDiv.innerHTML = LOADING_HTML;
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
                    updateMessageContent(assistantDiv, raw);
                    chatContainer.scrollTop = chatContainer.scrollHeight;
                }
            }
            // 流结束后，最后一次性渲染 Markdown
            updateMessageContent(assistantDiv, raw);
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

    // Chat File Upload Logic
    attachBtn.addEventListener('click', () => {
        chatFileInput.click();
    });

    chatFileInput.addEventListener('change', () => {
        const file = chatFileInput.files[0];
        filePreviewContainer.innerHTML = '';
        if (!file) {
            filePreviewContainer.style.display = 'none';
            return;
        }
        
        filePreviewContainer.style.display = 'flex';
        const item = document.createElement('div');
        item.className = 'file-preview-item';
        item.innerHTML = `
            <span>${file.name}</span>
            <button type="button" class="file-remove-btn" title="移除">&times;</button>
        `;
        
        item.querySelector('.file-remove-btn').addEventListener('click', () => {
            chatFileInput.value = '';
            filePreviewContainer.innerHTML = '';
            filePreviewContainer.style.display = 'none';
        });
        
        filePreviewContainer.appendChild(item);
    });

    async function sendMessage() {
        let message = messageInput.value.trim();
        const file = chatFileInput.files[0];
        
        if (!message && !file) return;
        
        sendBtn.disabled = true;
        attachBtn.disabled = true;

        // 1. Handle File Upload
        if (file) {
            const previewItem = filePreviewContainer.querySelector('.file-preview-item');
            if (previewItem) {
                previewItem.innerHTML = `<span>正在上传 ${file.name}...</span>`;
            }

            try {
                const formData = new FormData();
                formData.append('file', file);
                const res = await fetch('/api/kb/upload', {
                    method: 'POST',
                    body: formData
                });
                
                if (!res.ok) {
                    const t = await res.text();
                    throw new Error(t);
                }
                
                // Clear file input
                chatFileInput.value = '';
                filePreviewContainer.innerHTML = '';
                filePreviewContainer.style.display = 'none';
                
                // Append file info
                if (!message) {
                    message = `请分析上传的文件: [${file.name}]`;
                } else {
                    message += `\n\n[已上传文件: [${file.name}]`;
                }
            } catch (err) {
                console.error(err);
                alert('文件上传失败: ' + err.message);
                sendBtn.disabled = false;
                attachBtn.disabled = false;
                // Restore preview
                if (previewItem) {
                     previewItem.innerHTML = `
                        <span>${file.name} (上传失败)</span>
                        <button type="button" class="file-remove-btn" title="移除">&times;</button>
                    `;
                    previewItem.querySelector('.file-remove-btn').addEventListener('click', () => {
                        chatFileInput.value = '';
                        filePreviewContainer.innerHTML = '';
                        filePreviewContainer.style.display = 'none';
                    });
                }
                return;
            }
        }
        
        // 2. Auto-create conversation if needed
        if (!currentConversationId) {
            try {
                await createConversation();
                if (!currentConversationId) {
                    alert('无法创建新会话');
                    sendBtn.disabled = false;
                    attachBtn.disabled = false;
                    return;
                }
            } catch (err) {
                console.error('Auto-create conversation failed:', err);
                alert('自动创建会话失败');
                sendBtn.disabled = false;
                attachBtn.disabled = false;
                return;
            }
        }

        appendMessage('user', message);
        const assistantDiv = appendMessage('assistant', '');
        assistantDiv.innerHTML = LOADING_HTML;
        messageInput.value = '';
        
        // Adjust scroll immediately
        chatContainer.scrollTop = chatContainer.scrollHeight;

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
                    updateMessageContent(assistantDiv, raw);
                    chatContainer.scrollTop = chatContainer.scrollHeight;
                }
            }
            // 流结束后，最后一次性渲染 Markdown
            updateMessageContent(assistantDiv, raw);
            await refreshConversations();
        } catch (error) {
            console.error('Error:', error);
            assistantDiv.textContent = 'Error: ' + error.message;
        } finally {
            sendBtn.disabled = false;
            attachBtn.disabled = false;
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

        // 处理 think-title 点击
        if (el.classList.contains('think-title')) {
            const block = el.closest('.think-block');
            if (block) {
                block.classList.toggle('collapsed');
            }
            return;
        }

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
    // System prompt elements
    const systemPromptInput = document.getElementById('system-prompt-input');
    const saveSystemPromptBtn = document.getElementById('save-system-prompt');

    settingsBtn.addEventListener('click', () => {
        settingsModal.style.display = 'block';
        loadSettings();
        loadSystemPrompt();
        loadKBFiles();
        loadModels();
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

    async function loadSystemPrompt() {
        try {
            const res = await fetch('/api/settings/system-prompt');
            const data = await res.json();
            if (data.prompt) {
                systemPromptInput.value = data.prompt;
            }
        } catch (err) {
            console.error('Failed to load system prompt:', err);
        }
    }

    saveSystemPromptBtn.addEventListener('click', async () => {
        const prompt = systemPromptInput.value.trim();
        try {
            const res = await fetch('/api/settings/system-prompt', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ value: prompt })
            });
            if (res.ok) {
                alert('保存成功');
            } else {
                alert('保存失败');
            }
        } catch (err) {
            console.error('Failed to save system prompt:', err);
            alert('保存出错');
        }
    });

    async function loadModels() {
        try {
            const res = await fetch('/api/models');
            if (!res.ok) throw new Error('Failed to fetch models');
            const data = await res.json();
            
            modelSelect.innerHTML = '';
            if (data.models && Array.isArray(data.models)) {
                data.models.forEach(model => {
                    const option = document.createElement('option');
                    option.value = model;
                    option.textContent = model;
                    modelSelect.appendChild(option);
                });
            }
            
            if (data.current_model) {
                modelSelect.value = data.current_model;
            }
        } catch (err) {
            console.error('Failed to load models:', err);
        }
    }

    modelSelect.addEventListener('change', async function() {
        const model = this.value;
        const selectedOption = this.options[this.selectedIndex];
        const originalText = selectedOption.text;

        // Disable controls and show loading state
        this.disabled = true;
        if (closeSettings) closeSettings.disabled = true;
        selectedOption.text = `${originalText} (Switching...)`;

        try {
            const res = await fetch('/api/models/select', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ model: model })
            });
            
            if (res.ok) {
                console.log('Model switched to', model);
            } else {
                const t = await res.text();
                alert('Failed to switch model: ' + t);
            }
        } catch (err) {
            console.error('Error switching model:', err);
            alert('Error switching model');
        } finally {
            // Restore state
            selectedOption.text = originalText;
            this.disabled = false;
            if (closeSettings) closeSettings.disabled = false;
        }
    });

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

    // 获取同步进度
    async function getSyncProgress() {
        try {
            const res = await fetch('/api/kb/sync/progress');
            if (res.ok) {
                return await res.json();
            }
        } catch (err) {
            console.error('Failed to get sync progress:', err);
        }
        return null;
    }

    // 更新进度条
    function updateProgressBar(progress) {
        if (!progress) return;
        
        syncProgressBar.style.width = `${progress.progress}%`;
        syncProgressText.textContent = `${Math.round(progress.progress)}%`;
        syncCurrentFile.textContent = progress.current_file || '';
        
        // 根据状态更新文本
        switch (progress.status) {
            case 'scanning':
                syncStatus.textContent = '正在扫描文件...';
                break;
            case 'syncing':
                syncStatus.textContent = '正在同步文件...';
                break;
            case 'processing':
                syncStatus.textContent = '正在处理文件...';
                break;
            case 'completed':
                syncStatus.textContent = '同步完成';
                break;
            case 'scanned':
                syncStatus.textContent = '扫描完成，准备处理...';
                break;
            case 'idle':
                syncStatus.textContent = '就绪';
                break;
            default:
                syncStatus.textContent = progress.status || '正在同步...';
        }
        
        // 更新文件分片进度
        if (progress.chunk_progress && progress.chunk_progress.length > 0) {
            chunkProgressContainer.style.display = 'block';
            chunkProgressContent.innerHTML = '';
            
            progress.chunk_progress.forEach(chunk => {
                const chunkItem = document.createElement('div');
                chunkItem.className = 'chunk-progress-item';
                
                chunkItem.innerHTML = `
                    <div class="chunk-progress-header">
                        <span>${chunk.file_name}</span>
                        <span>${Math.round(chunk.progress)}%</span>
                    </div>
                    <div class="chunk-progress-bar">
                        <div class="chunk-progress-fill" style="width: ${chunk.progress}%"></div>
                    </div>
                    <div class="chunk-progress-info">
                        已处理 ${chunk.processed_chunks}/${chunk.total_chunks} 个分片
                    </div>
                `;
                
                chunkProgressContent.appendChild(chunkItem);
            });
        } else {
            chunkProgressContainer.style.display = 'none';
        }
    }

    syncKBBtn.addEventListener('click', async () => {
        if (syncKBBtn.disabled) return; // 防止重复点击
        
        syncStatus.textContent = '正在同步...';
        syncKBBtn.disabled = true;
        resetKBBtn.disabled = true;
        syncProgressContainer.style.display = 'block';
        
        try {
            const res = await fetch('/api/kb/sync', { method: 'POST' });
            if (res.ok) {
                syncStatus.textContent = '同步已开始';
                
                // 定期获取同步进度
                const progressTimer = setInterval(async () => {
                    const progress = await getSyncProgress();
                    updateProgressBar(progress);
                }, 1000);
                
                // 每隔几秒刷新一下文件列表
                const fileListTimer = setInterval(async () => {
                    const finished = await loadKBFiles();
                    if (finished) {
                        clearInterval(fileListTimer);
                        clearInterval(progressTimer);
                        
                        // 最后一次更新进度
                        const finalProgress = await getSyncProgress();
                        updateProgressBar(finalProgress);
                        
                        syncStatus.textContent = '同步完成';
                        syncKBBtn.disabled = false;
                        resetKBBtn.disabled = false;
                        
                        // 延迟隐藏进度条
                        setTimeout(() => {
                            syncProgressContainer.style.display = 'none';
                        }, 2000);
                    }
                }, 2000);
            } else {
                syncStatus.textContent = '同步失败';
                syncKBBtn.disabled = false;
                resetKBBtn.disabled = false;
                syncProgressContainer.style.display = 'none';
            }
        } catch (err) {
            console.error('Failed to sync KB:', err);
            syncStatus.textContent = '同步出错';
            syncKBBtn.disabled = false;
            resetKBBtn.disabled = false;
            syncProgressContainer.style.display = 'none';
        }
    });

    resetKBBtn.addEventListener('click', async () => {
        if (!confirm('确定要清空所有已导入的知识库文件吗？此操作不可恢复。')) {
            return;
        }
        
        syncStatus.textContent = '正在重置...';
        syncKBBtn.disabled = true;
        resetKBBtn.disabled = true;
        
        try {
            const res = await fetch('/api/kb/reset', { method: 'POST' });
            if (res.ok) {
                syncStatus.textContent = '知识库已清空';
                await loadKBFiles(); // 刷新列表，应该变空
            } else {
                const data = await res.json();
                syncStatus.textContent = '重置失败: ' + (data.error || '未知错误');
            }
        } catch (err) {
            console.error('Failed to reset KB:', err);
            syncStatus.textContent = '重置出错';
        } finally {
            syncKBBtn.disabled = false;
            resetKBBtn.disabled = false;
        }
    });

    uploadKBBtn.addEventListener('click', async () => {
        const file = kbFileUpload.files[0];
        if (!file) {
            alert('请先选择文件');
            return;
        }
        
        uploadKBBtn.disabled = true;
        uploadKBBtn.textContent = '上传中...';
        
        const formData = new FormData();
        formData.append('file', file);
        
        try {
            const res = await fetch('/api/kb/upload', {
                method: 'POST',
                body: formData
            });
            if (res.ok) {
                alert('上传成功并已开始处理');
                kbFileUpload.value = ''; // clear input
                await loadKBFiles();
            } else {
                const t = await res.text();
                alert('上传失败: ' + t);
            }
        } catch (err) {
            console.error('Upload failed:', err);
            alert('上传出错');
        } finally {
            uploadKBBtn.disabled = false;
            uploadKBBtn.textContent = '上传';
        }
    });

    // 预览相关函数
    function getFileExtension(filename) {
        return filename.slice((filename.lastIndexOf(".") - 1 >>> 0) + 2).toLowerCase();
    }

    async function openPreview(fileName) {
        previewModal.style.display = 'block';
        previewTitle.textContent = fileName;
        previewBody.innerHTML = '<div class="preview-loading"><div class="loading-dots"><div class="loading-dot"></div><div class="loading-dot"></div><div class="loading-dot"></div></div></div>';

        const ext = getFileExtension(fileName);
        const imageExts = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg'];

        // 1. 如果是图片，直接显示
        if (imageExts.includes(ext)) {
            const img = new Image();
            img.src = `/api/kb/download?file=${encodeURIComponent(fileName)}`;
            img.style.maxWidth = '100%';
            img.style.display = 'block';
            img.style.margin = '0 auto';
            img.onload = () => {
                previewBody.innerHTML = '';
                previewBody.appendChild(img);
            };
            img.onerror = () => {
                previewBody.innerHTML = '<div class="preview-error">图片加载失败</div>';
            };
            return;
        }

        // 2. 如果是 PDF，使用 iframe 预览
        if (ext === 'pdf') {
            previewBody.innerHTML = `<iframe src="/api/kb/download?file=${encodeURIComponent(fileName)}" style="width:100%; height:100%; min-height:500px; border:none;"></iframe>`;
            return;
        }

        // 2.5 Excel：用表格形式预览完整数据（后端流式输出 HTML）
        if (ext === 'xlsx' || ext === 'xls') {
            previewBody.innerHTML = `<iframe src="/api/kb/excel/preview?file=${encodeURIComponent(fileName)}" style="width:100%; height:100%; min-height:500px; border:none; border-radius: 10px; background: transparent;"></iframe>`;
            return;
        }

        // 3. 其他类型（Office、文本、代码），尝试获取解析后的文本内容
        try {
            const res = await fetch(`/api/kb/content?file=${encodeURIComponent(fileName)}`);
            if (!res.ok) throw new Error('Failed to load file content');
            const data = await res.json();
            
            if (data.content) {
                if (ext === 'md') {
                    // Markdown 渲染
                    previewBody.innerHTML = renderMarkdown(data.content);
                } else {
                    // 代码或纯文本显示
                    const pre = document.createElement('pre');
                    const code = document.createElement('code');
                    // 尝试匹配语言 class
                    code.className = `language-${ext}`;
                    code.textContent = data.content;
                    pre.appendChild(code);
                    previewBody.innerHTML = '';
                    previewBody.appendChild(pre);
                }
            } else {
                 throw new Error('No content available');
            }
        } catch (err) {
            console.error(err);
            previewBody.innerHTML = `
                <div class="preview-error" style="text-align: center; padding: 20px;">
                    <p style="margin-bottom: 15px;">无法直接预览此文件内容。</p>
                    <a href="/api/kb/download?file=${encodeURIComponent(fileName)}" target="_blank" style="display: inline-block; padding: 8px 16px; background: #3b82f6; color: white; text-decoration: none; border-radius: 4px;">下载查看</a>
                </div>`;
        }
    }

    closePreviewBtn.addEventListener('click', () => {
        previewModal.style.display = 'none';
    });

    window.addEventListener('click', (e) => {
        if (e.target === previewModal) {
            previewModal.style.display = 'none';
        }
    });

    async function loadKBFiles() {
        try {
            const res = await fetch('/api/kb/files');
            const rawFiles = await res.json();
            // 过滤掉以 ".~" 开头的临时文件
            const files = rawFiles.filter(f => {
                const fileName = f.Path.split('/').pop();
                return !fileName.startsWith('.~');
            });
            kbFileList.innerHTML = '';
            selectedFileIds.clear();
            updateFileBatchBtn();
            
            let allProcessed = true;
            if (files.length === 0) {
                kbFileList.innerHTML = '<div class="file-item">无文件</div>';
                return true;
            }

            // 获取同步进度，包含文件Chunk进度
            let syncProgress = null;
            try {
                const progressRes = await fetch('/api/kb/sync/progress');
                if (progressRes.ok) {
                    syncProgress = await progressRes.json();
                }
            } catch (err) {
                console.error('Failed to get sync progress:', err);
            }

            files.forEach(f => {
                const item = document.createElement('div');
                item.className = 'file-item';
                
                const fileName = f.Path.split('/').pop();
                const statusClass = `status-${f.Status}`;
                const statusText = f.Status === 'processed' ? '已处理' : 
                                  f.Status === 'pending' ? '处理中' : '错误';
                
                if (f.Status !== 'processed') allProcessed = false;

                // Checkbox
                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.className = 'file-checkbox';
                checkbox.dataset.id = f.ID;
                checkbox.addEventListener('change', () => {
                    if (checkbox.checked) {
                        selectedFileIds.add(f.ID);
                    } else {
                        selectedFileIds.delete(f.ID);
                    }
                    updateFileBatchBtn();
                });

                const left = document.createElement('div');
                left.className = 'file-item-left';
                left.appendChild(checkbox);
                
                const nameSpan = document.createElement('span');
                nameSpan.title = f.Path;
                nameSpan.textContent = fileName;
                nameSpan.style.cssText = 'overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-right: 10px; cursor: pointer; color: #3b82f6; text-decoration: underline;';
                nameSpan.addEventListener('click', (e) => {
                    e.stopPropagation();
                    openPreview(fileName);
                });
                left.appendChild(nameSpan);

                // 获取文件的Chunk进度
                let progressText = '';
                if (syncProgress && syncProgress.chunk_progress) {
                    const chunkProgress = syncProgress.chunk_progress.find(cp => cp.file_name === fileName);
                    if (chunkProgress) {
                        progressText = ` (${Math.round(chunkProgress.progress)}%)`;
                    }
                }

                const right = document.createElement('div');
                right.className = 'file-item-right';
                right.innerHTML = `
                    <span class="file-status ${statusClass}">${statusText}${progressText}</span>
                    <button class="file-delete-btn" style="background: none; border: none; cursor: pointer; color: #999; font-size: 1.2em; padding: 0 5px;">&times;</button>
                `;
                
                const deleteBtn = right.querySelector('.file-delete-btn');
                deleteBtn.addEventListener('click', async (e) => {
                    e.stopPropagation();
                    if (!confirm(`确定要删除文件 "${fileName}" 吗？\n这将同时删除磁盘上的物理文件。`)) return;
                      
                    try {
                        const res = await fetch(`/api/kb/files/${f.ID}`, { method: 'DELETE' });
                        if (res.ok) {
                            await loadKBFiles();
                        } else {
                            const data = await res.json();
                            alert('删除失败: ' + (data.error || 'Unknown error'));
                        }
                    } catch (err) {
                        console.error(err);
                        alert('删除出错');
                    }
                });
                
                item.appendChild(left);
                item.appendChild(right);
                kbFileList.appendChild(item);
            });
            return allProcessed;
        } catch (err) {
            console.error('Failed to load KB files:', err);
            return true;
        }
    }

    function updateFileBatchBtn() {
        if (selectedFileIds.size > 0) {
            deleteSelectedFilesBtn.style.display = 'block';
            deleteSelectedFilesBtn.textContent = `删除选中 (${selectedFileIds.size})`;
        } else {
            deleteSelectedFilesBtn.style.display = 'none';
        }
    }

    deleteSelectedFilesBtn.addEventListener('click', async () => {
        if (selectedFileIds.size === 0) return;
        if (!confirm(`确定要删除选中的 ${selectedFileIds.size} 个文件吗？\n这将同时删除磁盘上的物理文件。`)) return;

        try {
            const ids = Array.from(selectedFileIds);
            const res = await fetch('/api/kb/files/batch-delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ids })
            });
            
            if (!res.ok) {
                const t = await res.text();
                throw new Error(t);
            }

            await loadKBFiles();
        } catch (err) {
            console.error(err);
            alert('批量删除失败: ' + err.message);
        }
    });

    // 初始化
    loadConversations().catch(err => console.error('Failed to load conversations:', err));
});
