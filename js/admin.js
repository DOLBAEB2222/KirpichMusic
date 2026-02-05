class AdminPage extends Components.Component {
    constructor(props) {
        super(props);
        this.tab = 'users';
    }

    render() {
        if (!Auth.isAdmin()) {
            Utils.setHashUrl('/');
            return document.createElement('div');
        }

        const div = document.createElement('div');
        div.className = 'admin-page';
        
        div.innerHTML = `
            <div class="admin-sidebar">
                <div class="admin-logo">Админ-панель</div>
                <div class="admin-nav">
                    <button class="admin-nav-item ${this.tab === 'users' ? 'active' : ''}" data-tab="users">Пользователи</button>
                    <button class="admin-nav-item ${this.tab === 'tracks' ? 'active' : ''}" data-tab="tracks">Треки</button>
                    <button class="admin-nav-item ${this.tab === 'stats' ? 'active' : ''}" data-tab="stats">Статистика</button>
                    <button class="admin-nav-item ${this.tab === 'settings' ? 'active' : ''}" data-tab="settings">Настройки</button>
                </div>
                <div class="admin-footer">
                    <button class="btn btn-ghost" data-action="back">Назад в приложение</button>
                </div>
            </div>
            <div class="admin-content">
                <div class="admin-header">
                    <h1>${this.getTabTitle()}</h1>
                    <div class="admin-actions">
                        ${this.renderHeaderActions()}
                    </div>
                </div>
                <div class="admin-body">
                    ${this.renderTabContent()}
                </div>
            </div>
        `;

        div.addEventListener('click', (e) => {
            const tabBtn = e.target.closest('[data-tab]');
            if (tabBtn) {
                this.tab = tabBtn.dataset.tab;
                this.update();
                return;
            }

            const actionBtn = e.target.closest('[data-action]');
            if (actionBtn) {
                const action = actionBtn.dataset.action;
                if (action === 'back') {
                    Utils.setHashUrl('/');
                }
            }
        });

        return div;
    }

    getTabTitle() {
        switch (this.tab) {
            case 'users': return 'Управление пользователями';
            case 'tracks': return 'Модерация треков';
            case 'stats': return 'Системная статистика';
            case 'settings': return 'Настройки системы';
            default: return 'Администрирование';
        }
    }

    renderHeaderActions() {
        if (this.tab === 'users') {
            return `<button class="btn btn-primary">Добавить пользователя</button>`;
        }
        if (this.tab === 'tracks') {
            return `<button class="btn btn-primary">Загрузить трек</button>`;
        }
        return '';
    }

    renderTabContent() {
        switch (this.tab) {
            case 'users': return this.renderUsersTable();
            case 'tracks': return this.renderTracksTable();
            case 'stats': return this.renderStats();
            case 'settings': return this.renderSettings();
            default: return '';
        }
    }

    renderUsersTable() {
        const users = MockData.users;
        return `
            <table class="admin-table">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Пользователь</th>
                        <th>Роль</th>
                        <th>Статус</th>
                        <th>Действия</th>
                    </tr>
                </thead>
                <tbody>
                    ${users.map(user => `
                        <tr>
                            <td>${user.id}</td>
                            <td>
                                <div style="display: flex; align-items: center; gap: 10px;">
                                    <div class="avatar-sm" style="background: var(--color-accent-primary); width: 32px; height: 32px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-size: 12px;">${Utils.getInitials(user.displayName)}</div>
                                    <div>
                                        <div>${Utils.escapeHtml(user.displayName)}</div>
                                        <div style="font-size: 12px; color: var(--color-text-secondary);">@${user.username}</div>
                                    </div>
                                </div>
                            </td>
                            <td>${user.type === 'artist' ? 'Артист' : 'Слушатель'}</td>
                            <td><span class="badge badge-success">Активен</span></td>
                            <td>
                                <button class="btn btn-sm btn-ghost">Edit</button>
                                <button class="btn btn-sm btn-ghost" style="color: var(--color-error);">Ban</button>
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    }

    renderTracksTable() {
        const tracks = MockData.tracks;
        return `
            <table class="admin-table">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Название</th>
                        <th>Артист</th>
                        <th>Жанр</th>
                        <th>Прослушивания</th>
                        <th>Действия</th>
                    </tr>
                </thead>
                <tbody>
                    ${tracks.map(track => `
                        <tr>
                            <td>${track.id}</td>
                            <td>${Utils.escapeHtml(track.title)}</td>
                            <td>${Utils.escapeHtml(track.artist)}</td>
                            <td>${track.genre}</td>
                            <td>${Utils.formatNumber(track.plays)}</td>
                            <td>
                                <button class="btn btn-sm btn-ghost">Edit</button>
                                <button class="btn btn-sm btn-ghost" style="color: var(--color-error);">Delete</button>
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        `;
    }

    renderStats() {
        return `
            <div class="admin-stats-grid">
                <div class="admin-stat-card">
                    <div class="admin-stat-label">Всего пользователей</div>
                    <div class="admin-stat-value">${MockData.users.length}</div>
                </div>
                <div class="admin-stat-card">
                    <div class="admin-stat-label">Всего треков</div>
                    <div class="admin-stat-value">${MockData.tracks.length}</div>
                </div>
                <div class="admin-stat-card">
                    <div class="admin-stat-label">Активных сессий</div>
                    <div class="admin-stat-value">124</div>
                </div>
                <div class="admin-stat-card">
                    <div class="admin-stat-label">Загрузка CPU</div>
                    <div class="admin-stat-value">12%</div>
                </div>
            </div>
        `;
    }

    renderSettings() {
        return `
            <div class="admin-settings-form">
                <div class="form-group">
                    <label>Название сайта</label>
                    <input type="text" class="input" value="KirpichMusic">
                </div>
                <div class="form-group">
                    <label>Регистрация новых пользователей</label>
                    <select class="input">
                        <option value="enabled">Включена</option>
                        <option value="disabled">Отключена</option>
                        <option value="invite-only">Только по приглашениям</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>Максимальный размер трека (MB)</label>
                    <input type="number" class="input" value="50">
                </div>
                <button class="btn btn-primary">Сохранить изменения</button>
            </div>
        `;
    }
}

window.AdminPage = AdminPage;
