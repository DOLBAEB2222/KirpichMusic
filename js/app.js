class Router {
    constructor() {
        this.routes = new Map();
        this.currentPage = null;
        
        window.addEventListener('hashchange', () => this.handleRoute());
        window.addEventListener('load', () => this.handleRoute());
    }
    
    addRoute(path, handler) {
        this.routes.set(path, handler);
    }
    
    handleRoute() {
        const { path, params } = Utils.parseHashUrl();
        
        // Route protection
        if (path === '/admin' && !Auth.isAdmin()) {
            Utils.setHashUrl('/');
            return;
        }

        if (path !== '/' && path !== '/onboarding' && !store.state.isAuthenticated) {
            Utils.setHashUrl('/');
            return;
        }

        for (const [routePath, handler] of this.routes) {
            const match = this.matchRoute(routePath, path);
            if (match) {
                this.renderPage(handler, { ...match, ...params });
                return;
            }
        }
        
        this.renderPage(this.routes.get('/'), {});
    }
    
    matchRoute(routePath, path) {
        const routeParts = routePath.split('/').filter(Boolean);
        const pathParts = path.split('/').filter(Boolean);
        
        if (routeParts.length !== pathParts.length) {
            return null;
        }
        
        const params = {};
        for (let i = 0; i < routeParts.length; i++) {
            if (routeParts[i].startsWith(':')) {
                params[routeParts[i].slice(1)] = pathParts[i];
            } else if (routeParts[i] !== pathParts[i]) {
                return null;
            }
        }
        
        return params;
    }
    
    renderPage(handler, params) {
        if (this.currentPage && this.currentPage.unmount) {
            this.currentPage.unmount();
        }
        
        const app = document.getElementById('app');
        app.innerHTML = '';
        
        const pageContainer = document.createElement('div');
        pageContainer.className = 'page-transition';
        app.appendChild(pageContainer);

        this.currentPage = handler(params);
        
        if (this.currentPage && this.currentPage.mount) {
            this.currentPage.mount(pageContainer);
        }
        
        Utils.initGlobalPanels();
    }
}

class AuthPage extends Components.Component {
    constructor(props) {
        super(props);
        this.mode = 'guest';
    }
    
    render() {
        const div = document.createElement('div');
        div.className = 'auth-page';
        
        div.innerHTML = `
            <div class="auth-container">
                <div class="auth-logo">
                    <div class="auth-logo-text">🧱 KirpichMusic</div>
                    <div class="auth-logo-subtitle">Музыка индустриальной роскоши</div>
                </div>
                <div class="auth-card">
                    <form class="auth-form" id="auth-form">
                        ${this.mode === 'guest' ? this.renderGuestMode() : ''}
                        ${this.mode === 'login' ? this.renderLoginMode() : ''}
                        ${this.mode === 'register' ? this.renderRegisterMode() : ''}
                    </form>
                </div>
            </div>
        `;
        
        const form = div.querySelector('#auth-form');
        form.addEventListener('submit', (e) => {
            e.preventDefault();
            this.handleSubmit();
        });
        
        return div;
    }
    
    renderGuestMode() {
        return `
            <h2 style="text-align: center; margin-bottom: var(--space-lg);">Добро пожаловать</h2>
            <p style="text-align: center; color: var(--color-text-secondary); margin-bottom: var(--space-xl);">
                Войдите или продолжите как гость для доступа к миллионам треков
            </p>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
                Войти как гость
            </button>
            <div class="auth-divider">или</div>
            <button type="button" class="btn btn-secondary btn-lg" style="width: 100%;" data-action="show-login">
                Войти в аккаунт
            </button>
            <button type="button" class="btn btn-ghost btn-lg" style="width: 100%; margin-top: var(--space-md);" data-action="show-register">
                Создать аккаунт
            </button>
        `;
    }
    
    renderLoginMode() {
        return `
            <h2 style="margin-bottom: var(--space-xl);">Вход</h2>
            <div class="input-group" style="margin-bottom: var(--space-lg);">
                <input type="text" class="input" placeholder="Email или имя пользователя" name="username" required>
            </div>
            <div class="input-group" style="margin-bottom: var(--space-xl);">
                <input type="password" class="input" placeholder="Пароль" name="password" required>
            </div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
                Войти
            </button>
            <button type="button" class="btn btn-ghost" style="width: 100%; margin-top: var(--space-md);" data-action="show-guest">
                Назад
            </button>
        `;
    }
    
    renderRegisterMode() {
        return `
            <h2 style="margin-bottom: var(--space-xl);">Регистрация</h2>
            <div class="input-group" style="margin-bottom: var(--space-lg);">
                <input type="text" class="input" placeholder="Имя пользователя" name="username" required>
            </div>
            <div class="input-group" style="margin-bottom: var(--space-lg);">
                <input type="email" class="input" placeholder="Email" name="email" required>
            </div>
            <div class="input-group" style="margin-bottom: var(--space-xl);">
                <input type="password" class="input" placeholder="Пароль" name="password" required>
            </div>
            <button type="submit" class="btn btn-primary btn-lg" style="width: 100%;">
                Создать аккаунт
            </button>
            <button type="button" class="btn btn-ghost" style="width: 100%; margin-top: var(--space-md);" data-action="show-guest">
                Назад
            </button>
        `;
    }
    
    onMount() {
        this.el.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            if (action === 'show-login') this.mode = 'login';
            else if (action === 'show-register') this.mode = 'register';
            else if (action === 'show-guest') this.mode = 'guest';
            
            this.update();
        });
    }
    
    async handleSubmit() {
        const form = this.el.querySelector('#auth-form');
        const formData = new FormData(form);
        const username = formData.get('username');
        const password = formData.get('password');

        if (this.mode === 'guest') {
            AppActions.loginAsGuest();
            Utils.setHashUrl('/onboarding');
        } else if (this.mode === 'login') {
            try {
                const user = await Auth.login(username, password);
                AppActions.login(user);
                Utils.setHashUrl('/onboarding');
            } catch (err) {
                alert('Ошибка входа: ' + err.message);
            }
        } else if (this.mode === 'register') {
            const user = {
                id: Utils.randomInt(100, 999),
                username: username || 'new_user',
                displayName: username || 'Новый пользователь',
                avatar: null,
                bio: 'Люблю музыку',
                verified: false,
                role: 'user',
                type: 'listener',
                followers: 0,
                following: 0,
                tracks: 0,
                playlists: 0
            };
            AppActions.login(user);
            Utils.setHashUrl('/onboarding');
        }
    }
}

class OnboardingPage extends Components.Component {
    constructor(props) {
        super(props);
        this.step = 0;
        this.steps = [
            {
                icon: '🎵',
                title: 'Добро пожаловать в KirpichMusic',
                description: 'Откройте для себя мир индустриальной музыки и электроники. Миллионы треков от талантливых артистов со всего мира.'
            },
            {
                icon: '🔊',
                title: 'Создавайте свои плейлисты',
                description: 'Собирайте любимые треки в персональные коллекции. Делитесь ими с друзьями и открывайте новое.'
            },
            {
                icon: '💬',
                title: 'Общайтесь в IRC чате',
                description: 'Присоединяйтесь к сообществу меломанов и музыкантов. Обсуждайте музыку, делитесь опытом и находите единомышленников.'
            }
        ];
    }
    
    render() {
        const step = this.steps[this.step];
        const div = document.createElement('div');
        div.className = 'onboarding-page';
        
        div.innerHTML = `
            <div class="onboarding-step">
                <div class="onboarding-icon">${step.icon}</div>
                <h1 class="onboarding-title">${step.title}</h1>
                <p class="onboarding-description">${step.description}</p>
                <div class="onboarding-actions">
                    ${this.step > 0 ? '<button class="btn btn-secondary btn-lg" data-action="prev">Назад</button>' : ''}
                    <button class="btn btn-primary btn-lg" data-action="next">
                        ${this.step === this.steps.length - 1 ? 'Начать' : 'Далее'}
                    </button>
                </div>
                <div class="onboarding-progress">
                    ${this.steps.map((_, i) => `
                        <div class="onboarding-dot ${i === this.step ? 'active' : ''}"></div>
                    `).join('')}
                </div>
            </div>
        `;
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            if (action === 'next') {
                if (this.step === this.steps.length - 1) {
                    AppActions.completeOnboarding();
                    Utils.setHashUrl('/');
                } else {
                    this.step++;
                    this.update();
                }
            } else if (action === 'prev') {
                this.step--;
                this.update();
            }
        });
        
        return div;
    }
}

class HomePage extends Components.Component {
    render() {
        const div = document.createElement('div');
        div.className = 'app-container';
        
        const featuredTracks = MockData.tracks.slice(0, 6);
        const trendingArtists = MockData.users.filter(u => u.type === 'artist').slice(0, 6);
        const popularPlaylists = MockData.playlists.slice(0, 6);
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="content-header">
                        <div class="content-header-top">
                            <h1>Главная</h1>
                            <div class="content-header-actions">
                                <button class="btn btn-icon btn-ghost" data-action="notifications">
                                    ${new Components.Icon({ name: 'bell', size: 20 }).render().outerHTML}
                                </button>
                                <button class="btn btn-icon btn-ghost" data-action="profile">
                                    ${new Components.Icon({ name: 'user', size: 20 }).render().outerHTML}
                                </button>
                            </div>
                        </div>
                    </div>
                    <div class="content-body">
                        <div class="hero">
                            <div class="hero-content">
                                <div class="hero-badge">
                                    <span>🎵</span>
                                    <span>Новое на KirpichMusic</span>
                                </div>
                                <h1 class="hero-title">Индустриальная роскошь</h1>
                                <p class="hero-description">
                                    Погрузитесь в мир тяжёлой электроники, индастриала и экспериментальных звуков. 
                                    Открывайте новых артистов и создавайте собственные коллекции.
                                </p>
                                <div class="hero-actions">
                                    <button class="btn btn-primary btn-lg" data-action="play-featured">
                                        ${new Components.Icon({ name: 'play', size: 20 }).render().outerHTML}
                                        Воспроизвести
                                    </button>
                                    <button class="btn btn-secondary btn-lg" data-action="explore">
                                        Исследовать
                                    </button>
                                </div>
                            </div>
                        </div>
                        
                        <div class="section">
                            <div class="section-header">
                                <h2 class="section-title">Избранные треки</h2>
                                <a href="#/browse/tracks" class="section-link">Показать все</a>
                            </div>
                            <div class="grid grid-auto-fill" id="featured-tracks"></div>
                        </div>
                        
                        <div class="section">
                            <div class="section-header">
                                <h2 class="section-title">Популярные артисты</h2>
                                <a href="#/browse/artists" class="section-link">Показать все</a>
                            </div>
                            <div class="grid grid-auto-fill" id="trending-artists"></div>
                        </div>
                        
                        <div class="section">
                            <div class="section-header">
                                <h2 class="section-title">Рекомендованные плейлисты</h2>
                                <a href="#/browse/playlists" class="section-link">Показать все</a>
                            </div>
                            <div class="grid grid-auto-fill" id="popular-playlists"></div>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        const featuredTracksEl = div.querySelector('#featured-tracks');
        featuredTracks.forEach(track => {
            new Components.TrackCard({ track }).mount(featuredTracksEl);
        });
        
        const trendingArtistsEl = div.querySelector('#trending-artists');
        trendingArtists.forEach(artist => {
            new Components.ArtistCard({ artist }).mount(trendingArtistsEl);
        });
        
        const popularPlaylistsEl = div.querySelector('#popular-playlists');
        popularPlaylists.forEach(playlist => {
            new Components.PlaylistCard({ playlist }).mount(popularPlaylistsEl);
        });
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            if (action === 'play-featured') {
                AppActions.playTrack(featuredTracks[0], featuredTracks, 0);
            } else if (action === 'explore') {
                Utils.setHashUrl('/radio');
            } else if (action === 'profile') {
                Utils.setHashUrl(`/profile/${store.state.currentUser.id}`);
            } else if (action === 'notifications') {
                AppActions.showNotification({
                    title: 'Уведомления',
                    message: 'У вас нет новых уведомлений'
                });
            }
        });
        
        return div;
    }
}

class ProfilePage extends Components.Component {
    render() {
        const userId = parseInt(this.props.id);
        const user = MockData.users.find(u => u.id === userId);
        
        if (!user) {
            return document.createElement('div');
        }
        
        const userTracks = MockData.tracks.filter(t => t.artistId === userId);
        const isFollowing = AppActions.isFollowing(userId);
        const isOwnProfile = store.state.currentUser?.id === userId;
        const initials = Utils.getInitials(user.displayName);
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="profile-header">
                        <div class="profile-cover">
                            <div class="profile-avatar" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 4rem; font-weight: 700;">
                                ${initials}
                            </div>
                            ${user.verified ? `
                                <div class="profile-verified">
                                    ${new Components.Icon({ name: 'check', size: 16 }).render().outerHTML}
                                </div>
                            ` : ''}
                        </div>
                        <div class="profile-info">
                            <div class="profile-name">${Utils.escapeHtml(user.displayName)}</div>
                            <div class="profile-username">@${Utils.escapeHtml(user.username)}</div>
                            ${user.bio ? `<div class="profile-bio">${Utils.escapeHtml(user.bio)}</div>` : ''}
                            <div class="profile-stats">
                                <div class="profile-stat">
                                    <div class="profile-stat-value">${Utils.formatNumber(user.followers)}</div>
                                    <div class="profile-stat-label">Подписчиков</div>
                                </div>
                                <div class="profile-stat">
                                    <div class="profile-stat-value">${Utils.formatNumber(user.following)}</div>
                                    <div class="profile-stat-label">Подписок</div>
                                </div>
                                ${user.type === 'artist' ? `
                                    <div class="profile-stat">
                                        <div class="profile-stat-value">${user.tracks}</div>
                                        <div class="profile-stat-label">Треков</div>
                                    </div>
                                ` : ''}
                            </div>
                            <div class="profile-actions">
                                ${!isOwnProfile ? `
                                    <button class="btn ${isFollowing ? 'btn-secondary' : 'btn-primary'}" data-action="follow">
                                        ${isFollowing ? 'Отписаться' : 'Подписаться'}
                                    </button>
                                ` : ''}
                                <button class="btn btn-secondary" data-action="share">
                                    ${new Components.Icon({ name: 'share', size: 20 }).render().outerHTML}
                                    Поделиться
                                </button>
                            </div>
                        </div>
                    </div>
                    <div class="content-body">
                        ${userTracks.length > 0 ? `
                            <div class="section">
                                <div class="section-header">
                                    <h2 class="section-title">Треки</h2>
                                </div>
                                <div class="list" id="user-tracks"></div>
                            </div>
                        ` : ''}
                    </div>
                </div>
            </div>
        `;
        
        const userTracksEl = div.querySelector('#user-tracks');
        if (userTracksEl) {
            userTracks.forEach(track => {
                const item = document.createElement('div');
                item.className = 'list-item';
                item.innerHTML = `
                    <div class="list-item-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 1.5rem;">
                        🎵
                    </div>
                    <div class="list-item-content">
                        <div class="list-item-title">${Utils.escapeHtml(track.title)}</div>
                        <div class="list-item-subtitle">${Utils.formatNumber(track.plays)} прослушиваний</div>
                    </div>
                    <div class="list-item-actions">
                        <span class="text-secondary text-sm">${Utils.formatTime(track.duration)}</span>
                        <button class="btn btn-icon btn-ghost">
                            ${new Components.Icon({ name: 'more-horizontal', size: 20 }).render().outerHTML}
                        </button>
                    </div>
                `;
                item.addEventListener('click', () => {
                    AppActions.playTrack(track, userTracks, userTracks.indexOf(track));
                });
                userTracksEl.appendChild(item);
            });
        }
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            if (action === 'follow') {
                AppActions.toggleFollowArtist(userId);
                this.update();
            } else if (action === 'share') {
                AppActions.showNotification({
                    title: 'Профиль скопирован',
                    message: 'Ссылка скопирована в буфер обмена'
                });
            }
        });
        
        return div;
    }
}

class PlaylistPage extends Components.Component {
    render() {
        const playlistId = this.props.id;
        const allPlaylists = [...MockData.playlists, ...store.state.userPlaylists];
        const playlist = allPlaylists.find(p => p.id == playlistId);
        
        if (!playlist) {
            return document.createElement('div');
        }
        
        const tracks = MockData.tracks.filter(t => playlist.tracks.includes(t.id));
        const totalDuration = tracks.reduce((sum, t) => sum + t.duration, 0);
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="playlist-header">
                        <div class="playlist-cover" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 5rem;">
                            📀
                        </div>
                        <div class="playlist-info">
                            <div class="playlist-type">Плейлист</div>
                            <h1 class="playlist-title">${Utils.escapeHtml(playlist.title)}</h1>
                            ${playlist.description ? `<p class="playlist-description">${Utils.escapeHtml(playlist.description)}</p>` : ''}
                            <div class="playlist-meta">
                                <span>${Utils.escapeHtml(playlist.owner)}</span>
                                <span class="playlist-meta-separator"></span>
                                <span>${tracks.length} треков</span>
                                <span class="playlist-meta-separator"></span>
                                <span>${Utils.formatTime(totalDuration)}</span>
                            </div>
                        </div>
                    </div>
                    <div class="playlist-actions">
                        <button class="btn btn-primary btn-lg" data-action="play-all">
                            ${new Components.Icon({ name: 'play', size: 24 }).render().outerHTML}
                            Воспроизвести
                        </button>
                        <button class="btn btn-icon btn-ghost btn-lg">
                            ${new Components.Icon({ name: 'heart', size: 24 }).render().outerHTML}
                        </button>
                        <button class="btn btn-icon btn-ghost btn-lg">
                            ${new Components.Icon({ name: 'more-horizontal', size: 24 }).render().outerHTML}
                        </button>
                    </div>
                    <div class="playlist-tracks">
                        <div class="track-list-header">
                            <div>#</div>
                            <div>Название</div>
                            <div>Альбом</div>
                            <div>Добавлено</div>
                            <div>${new Components.Icon({ name: 'clock', size: 16 }).render().outerHTML}</div>
                        </div>
                        <div id="track-list"></div>
                    </div>
                </div>
            </div>
        `;
        
        const trackList = div.querySelector('#track-list');
        tracks.forEach((track, index) => {
            const item = document.createElement('div');
            item.className = 'track-list-item';
            item.innerHTML = `
                <div class="track-number">${index + 1}</div>
                <div class="track-info">
                    <div class="track-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white;">
                        🎵
                    </div>
                    <div class="track-details">
                        <div class="track-name truncate">${Utils.escapeHtml(track.title)}</div>
                        <div class="track-artist truncate">${Utils.escapeHtml(track.artist)}</div>
                    </div>
                </div>
                <div class="track-album truncate">${Utils.escapeHtml(track.album)}</div>
                <div class="text-secondary text-sm">Недавно</div>
                <div class="track-duration">${Utils.formatTime(track.duration)}</div>
            `;
            item.addEventListener('click', () => {
                AppActions.playTrack(track, tracks, index);
            });
            trackList.appendChild(item);
        });
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            if (action === 'play-all') {
                AppActions.playTrack(tracks[0], tracks, 0);
            }
        });
        
        return div;
    }
}

class ChatPage extends Components.Component {
    constructor(props) {
        super(props);
        this.messageInput = null;
    }
    
    render() {
        const { currentIrcChannel, unreadMessages } = store.state;
        const channels = ['#general', '#music-production', '#showcase', '#random', '#support'];
        const messages = MockData.ircMessages.filter(m => m.channel === currentIrcChannel);
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="chat-container draggable-panel" id="chat-panel">
                        <div class="panel-handle chat-drag-handle">:::</div>
                        <div class="chat-layout">
                            <div class="chat-channels">
                                <h3 style="margin-bottom: var(--space-lg); font-size: var(--font-size-sm); text-transform: uppercase; color: var(--color-text-tertiary);">Каналы</h3>
                                ${channels.map(channel => `
                                    <div class="chat-channel-item ${channel === currentIrcChannel ? 'active' : ''}" data-channel="${channel}">
                                        <span class="chat-channel-icon">#</span>
                                        <span>${channel.slice(1)}</span>
                                        ${unreadMessages[channel] ? `<span class="chat-channel-unread">${unreadMessages[channel]}</span>` : ''}
                                    </div>
                                `).join('')}
                            </div>
                            <div class="chat-main">
                                <div class="chat-header">
                                    <div>
                                        <h2 style="margin: 0;">${currentIrcChannel}</h2>
                                        <p style="color: var(--color-text-secondary); font-size: var(--font-size-sm); margin: 0;">
                                            ${messages.length} сообщений
                                        </p>
                                    </div>
                                </div>
                                <div class="chat-messages" id="chat-messages">
                                    ${messages.map(msg => this.renderMessage(msg)).join('')}
                                </div>
                                <div class="chat-input-wrapper">
                                    <textarea class="chat-input" placeholder="Написать сообщение..." id="chat-input"></textarea>
                                </div>
                            </div>
                        </div>
                        <div class="panel-resizer chat-resizer"></div>
                    </div>
                </div>
            </div>
        `;
        
        div.addEventListener('click', (e) => {
            const channelItem = e.target.closest('.chat-channel-item');
            if (channelItem) {
                const channel = channelItem.dataset.channel;
                AppActions.setIrcChannel(channel);
                this.update();
                return;
            }
            
            const mention = e.target.closest('.chat-message-mention');
            if (mention) {
                const username = mention.textContent.slice(1);
                const user = MockData.users.find(u => u.username === username);
                if (user) {
                    Utils.setHashUrl(`/profile/${user.id}`);
                }
                return;
            }
            
            const author = e.target.closest('.chat-message-author');
            if (author) {
                const userId = parseInt(author.dataset.userId);
                Utils.setHashUrl(`/profile/${userId}`);
            }
        });
        
        return div;
    }
    
    renderMessage(msg) {
        const initials = Utils.getInitials(msg.displayName);
        const text = this.formatMessageText(msg.text);
        
        return `
            <div class="chat-message">
                <div class="chat-message-avatar">
                    <div class="avatar" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);">
                        ${initials}
                    </div>
                </div>
                <div class="chat-message-content">
                    <div class="chat-message-header">
                        <span class="chat-message-author" data-user-id="${msg.userId}">${Utils.escapeHtml(msg.displayName)}</span>
                        <span class="chat-message-time">${Utils.formatDate(msg.timestamp)}</span>
                    </div>
                    <div class="chat-message-text">${text}</div>
                </div>
            </div>
        `;
    }
    
    formatMessageText(text) {
        return Utils.escapeHtml(text).replace(/@(\w+)/g, '<span class="chat-message-mention">@$1</span>');
    }
    
    onMount() {
        this.messageInput = this.el.querySelector('#chat-input');
        
        const chatPanel = this.el.querySelector('#chat-panel');
        if (chatPanel) {
            Panels.initDraggable(chatPanel, '.chat-drag-handle', 'chat');
            Panels.initResizable(chatPanel, '.chat-resizer', 'chat', { minWidth: 400, minHeight: 300 });
        }

        if (this.messageInput) {
            this.messageInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    this.sendMessage();
                }
            });
        }
        
        const messagesEl = this.el.querySelector('#chat-messages');
        if (messagesEl) {
            messagesEl.scrollTop = messagesEl.scrollHeight;
        }
    }
    
    sendMessage() {
        const text = this.messageInput.value.trim();
        if (!text) return;
        
        const user = store.state.currentUser;
        const message = {
            id: MockData.ircMessages.length + 1,
            userId: user.id,
            username: user.username,
            displayName: user.displayName,
            avatar: user.avatar,
            channel: store.state.currentIrcChannel,
            text,
            timestamp: new Date().toISOString()
        };
        
        AppActions.addIrcMessage(message);
        this.messageInput.value = '';
        this.update();
        
        setTimeout(() => {
            const messagesEl = this.el.querySelector('#chat-messages');
            if (messagesEl) {
                messagesEl.scrollTop = messagesEl.scrollHeight;
            }
        }, 0);
    }
}

class LikedPage extends Components.Component {
    render() {
        const likedTracks = store.state.likedTracks;
        const totalDuration = likedTracks.reduce((sum, t) => sum + t.duration, 0);
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="playlist-header">
                        <div class="playlist-cover" style="background: linear-gradient(135deg, #e63946 0%, #f1faee 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 5rem;">
                            ❤️
                        </div>
                        <div class="playlist-info">
                            <div class="playlist-type">Коллекция</div>
                            <h1 class="playlist-title">Понравившиеся треки</h1>
                            <div class="playlist-meta">
                                <span>${likedTracks.length} треков</span>
                                ${likedTracks.length > 0 ? `
                                    <span class="playlist-meta-separator"></span>
                                    <span>${Utils.formatTime(totalDuration)}</span>
                                ` : ''}
                            </div>
                        </div>
                    </div>
                    ${likedTracks.length > 0 ? `
                        <div class="playlist-actions">
                            <button class="btn btn-primary btn-lg" data-action="play-all">
                                ${new Components.Icon({ name: 'play', size: 24 }).render().outerHTML}
                                Воспроизвести
                            </button>
                            <button class="btn btn-secondary btn-lg" data-action="shuffle">
                                ${new Components.Icon({ name: 'shuffle', size: 24 }).render().outerHTML}
                                Перемешать
                            </button>
                        </div>
                        <div class="playlist-tracks">
                            <div class="track-list-header">
                                <div>#</div>
                                <div>Название</div>
                                <div>Альбом</div>
                                <div>Добавлено</div>
                                <div>${new Components.Icon({ name: 'clock', size: 16 }).render().outerHTML}</div>
                            </div>
                            <div id="track-list"></div>
                        </div>
                    ` : `
                        <div class="content-body">
                            <div style="text-align: center; padding: var(--space-xxxl); color: var(--color-text-secondary);">
                                <div style="font-size: 4rem; margin-bottom: var(--space-lg);">❤️</div>
                                <h3 style="margin-bottom: var(--space-md);">Нет понравившихся треков</h3>
                                <p>Нажимайте на сердечко, чтобы добавлять треки в избранное</p>
                            </div>
                        </div>
                    `}
                </div>
            </div>
        `;
        
        const trackList = div.querySelector('#track-list');
        if (trackList) {
            likedTracks.forEach((track, index) => {
                const item = document.createElement('div');
                item.className = 'track-list-item';
                item.innerHTML = `
                    <div class="track-number">${index + 1}</div>
                    <div class="track-info">
                        <div class="track-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white;">
                            🎵
                        </div>
                        <div class="track-details">
                            <div class="track-name truncate">${Utils.escapeHtml(track.title)}</div>
                            <div class="track-artist truncate">${Utils.escapeHtml(track.artist)}</div>
                        </div>
                    </div>
                    <div class="track-album truncate">${Utils.escapeHtml(track.album)}</div>
                    <div class="text-secondary text-sm">Недавно</div>
                    <div class="track-duration">${Utils.formatTime(track.duration)}</div>
                `;
                item.addEventListener('click', () => {
                    AppActions.playTrack(track, likedTracks, index);
                });
                trackList.appendChild(item);
            });
        }
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            if (action === 'play-all') {
                AppActions.playTrack(likedTracks[0], likedTracks, 0);
            } else if (action === 'shuffle') {
                const shuffled = Utils.shuffle(likedTracks);
                AppActions.playTrack(shuffled[0], shuffled, 0);
            }
        });
        
        return div;
    }
    
    onMount() {
        this.unsubscribers.push(
            store.subscribe('likedTracks', () => this.update())
        );
    }
}

class HistoryPage extends Components.Component {
    render() {
        const history = store.state.history;
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="content-header">
                        <div class="content-header-top">
                            <h1>История прослушиваний</h1>
                        </div>
                    </div>
                    <div class="content-body">
                        ${history.length > 0 ? `
                            <div class="list" id="history-list"></div>
                        ` : `
                            <div style="text-align: center; padding: var(--space-xxxl); color: var(--color-text-secondary);">
                                <div style="font-size: 4rem; margin-bottom: var(--space-lg);">🕐</div>
                                <h3 style="margin-bottom: var(--space-md);">История пуста</h3>
                                <p>Начните слушать музыку, чтобы увидеть историю</p>
                            </div>
                        `}
                    </div>
                </div>
            </div>
        `;
        
        const historyList = div.querySelector('#history-list');
        if (historyList) {
            history.forEach(track => {
                const item = document.createElement('div');
                item.className = 'list-item';
                item.innerHTML = `
                    <div class="list-item-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 1.5rem;">
                        🎵
                    </div>
                    <div class="list-item-content">
                        <div class="list-item-title">${Utils.escapeHtml(track.title)}</div>
                        <div class="list-item-subtitle">${Utils.escapeHtml(track.artist)} • ${Utils.formatDate(track.playedAt)}</div>
                    </div>
                    <div class="list-item-actions">
                        <span class="text-secondary text-sm">${Utils.formatTime(track.duration)}</span>
                    </div>
                `;
                item.addEventListener('click', () => {
                    AppActions.playTrack(track);
                });
                historyList.appendChild(item);
            });
        }
        
        return div;
    }
}

class RadioPage extends Components.Component {
    render() {
        const genres = ['Industrial', 'Electronica', 'Techno', 'Synthwave', 'Bass', 'Ambient', 'Cyberpunk'];
        const randomTracks = Utils.shuffle(MockData.tracks).slice(0, 20);
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="app-main">
                ${new Components.Sidebar().render().outerHTML}
                <div class="content">
                    <div class="content-header">
                        <div class="content-header-top">
                            <h1>Радио и открытия</h1>
                        </div>
                    </div>
                    <div class="content-body">
                        <div class="hero" style="margin-bottom: var(--space-xxxl);">
                            <div class="hero-content">
                                <div class="hero-badge">
                                    <span>📻</span>
                                    <span>Режим радио</span>
                                </div>
                                <h1 class="hero-title">Открывайте новую музыку</h1>
                                <p class="hero-description">
                                    Позвольте алгоритму подобрать для вас идеальную подборку треков на основе ваших предпочтений
                                </p>
                                <div class="hero-actions">
                                    <button class="btn btn-primary btn-lg" data-action="start-radio">
                                        ${new Components.Icon({ name: 'radio', size: 20 }).render().outerHTML}
                                        Запустить радио
                                    </button>
                                </div>
                            </div>
                        </div>
                        
                        <div class="section">
                            <div class="section-header">
                                <h2 class="section-title">Жанры</h2>
                            </div>
                            <div class="grid grid-cols-3" style="gap: var(--space-md);">
                                ${genres.map(genre => `
                                    <button class="btn btn-secondary btn-lg" data-genre="${genre}" style="height: 100px; font-size: var(--font-size-lg);">
                                        ${genre}
                                    </button>
                                `).join('')}
                            </div>
                        </div>
                        
                        <div class="section">
                            <div class="section-header">
                                <h2 class="section-title">Рекомендации для вас</h2>
                            </div>
                            <div class="grid grid-auto-fill" id="recommendations"></div>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        const recommendationsEl = div.querySelector('#recommendations');
        randomTracks.forEach(track => {
            new Components.TrackCard({ track }).mount(recommendationsEl);
        });
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (target && target.dataset.action === 'start-radio') {
                AppActions.playTrack(randomTracks[0], randomTracks, 0);
                AppActions.showNotification({
                    title: 'Радио запущено',
                    message: 'Наслаждайтесь подборкой'
                });
            }
            
            const genreBtn = e.target.closest('[data-genre]');
            if (genreBtn) {
                const genre = genreBtn.dataset.genre;
                const genreTracks = MockData.tracks.filter(t => t.genre === genre);
                if (genreTracks.length > 0) {
                    AppActions.playTrack(genreTracks[0], genreTracks, 0);
                }
            }
        });
        
        return div;
    }
}

class PlayerPage extends Components.Component {
    render() {
        const { currentTrack, queue, queueIndex } = store.state;
        
        if (!currentTrack) {
            Utils.setHashUrl('/');
            return document.createElement('div');
        }
        
        const div = document.createElement('div');
        div.className = 'app-container';
        
        div.innerHTML = `
            <div class="expanded-player">
                <div class="expanded-player-main">
                    <div class="expanded-player-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 6rem;">
                        🎵
                    </div>
                    <div class="expanded-player-info">
                        <h1 class="expanded-player-title">${Utils.escapeHtml(currentTrack.title)}</h1>
                        <p class="expanded-player-artist">${Utils.escapeHtml(currentTrack.artist)}</p>
                    </div>
                </div>
                <div class="expanded-player-sidebar">
                    <div class="expanded-player-tabs">
                        <button class="expanded-player-tab active" data-tab="queue">Очередь</button>
                        <button class="expanded-player-tab" data-tab="lyrics">Текст</button>
                    </div>
                    <div class="expanded-player-content" id="player-content">
                        ${this.renderQueue(queue, queueIndex)}
                    </div>
                </div>
            </div>
        `;
        
        div.addEventListener('click', (e) => {
            const tab = e.target.closest('.expanded-player-tab');
            if (tab) {
                div.querySelectorAll('.expanded-player-tab').forEach(t => t.classList.remove('active'));
                tab.classList.add('active');
                
                const content = div.querySelector('#player-content');
                if (tab.dataset.tab === 'queue') {
                    content.innerHTML = this.renderQueue(queue, queueIndex);
                } else if (tab.dataset.tab === 'lyrics') {
                    content.innerHTML = this.renderLyrics();
                }
            }
            
            const queueItem = e.target.closest('.queue-item');
            if (queueItem) {
                const index = parseInt(queueItem.dataset.index);
                AppActions.playTrack(queue[index], queue, index);
            }
        });
        
        return div;
    }
    
    renderQueue(queue, queueIndex) {
        return queue.map((track, index) => `
            <div class="queue-item ${index === queueIndex ? 'playing' : ''}" data-index="${index}">
                <div class="queue-item-number">${index + 1}</div>
                <div class="list-item-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white;">
                    🎵
                </div>
                <div class="list-item-content">
                    <div class="list-item-title">${Utils.escapeHtml(track.title)}</div>
                    <div class="list-item-subtitle">${Utils.escapeHtml(track.artist)}</div>
                </div>
                <div class="text-secondary text-sm">${Utils.formatTime(track.duration)}</div>
            </div>
        `).join('');
    }
    
    renderLyrics() {
        const sampleLyrics = [
            'Металлический пульс бьётся в ритме машин',
            'Индустриальный город никогда не спит',
            'Электрические провода тянутся к небесам',
            'Мы танцуем под звуки заводских турбин',
            '',
            'Стальные конструкции возвышаются над нами',
            'Неоновый свет освещает тёмные улицы',
            'Мы дети эпохи, где человек и машина едины',
            'Бетонные джунгли - наш новый дом',
            '',
            'В этом городе нет места тишине',
            'Только бесконечный гул и вибрация',
            'Мы находим красоту в индустриальном хаосе',
            'Это наш мир, наша музыка, наша жизнь'
        ];
        
        return `
            <div class="lyrics">
                ${sampleLyrics.map((line, i) => `
                    <div class="lyrics-line ${i === 0 ? 'active' : ''}">${line || '<br>'}</div>
                `).join('')}
            </div>
        `;
    }
    
    onMount() {
        this.unsubscribers.push(
            store.subscribe('currentTrack', () => this.update()),
            store.subscribe('queue', () => this.update())
        );
    }
}

class SearchModal extends Components.Component {
    render() {
        const { isSearchOpen, searchQuery, searchResults } = store.state;
        
        if (!isSearchOpen) {
            return document.createElement('div');
        }
        
        const div = document.createElement('div');
        div.className = 'modal-backdrop';
        
        div.innerHTML = `
            <div class="search-command">
                <div class="search-input-wrapper">
                    <input type="text" class="search-input" placeholder="Поиск треков, артистов, плейлистов..." value="${Utils.escapeHtml(searchQuery)}" id="search-input" autofocus>
                </div>
                ${searchResults ? `
                    <div class="search-results">
                        ${searchResults.tracks.length > 0 ? `
                            <div class="search-category">
                                <div class="search-category-title">Треки</div>
                                ${searchResults.tracks.map(track => `
                                    <div class="search-result-item" data-type="track" data-id="${track.id}">
                                        <div class="list-item-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white;">
                                            🎵
                                        </div>
                                        <div class="list-item-content">
                                            <div class="list-item-title">${Utils.escapeHtml(track.title)}</div>
                                            <div class="list-item-subtitle">${Utils.escapeHtml(track.artist)}</div>
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        ` : ''}
                        ${searchResults.artists.length > 0 ? `
                            <div class="search-category">
                                <div class="search-category-title">Артисты</div>
                                ${searchResults.artists.map(artist => {
                                    const initials = Utils.getInitials(artist.displayName);
                                    return `
                                        <div class="search-result-item" data-type="artist" data-id="${artist.id}">
                                            <div class="avatar" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);">
                                                ${initials}
                                            </div>
                                            <div class="list-item-content">
                                                <div class="list-item-title">${Utils.escapeHtml(artist.displayName)}</div>
                                                <div class="list-item-subtitle">Артист</div>
                                            </div>
                                        </div>
                                    `;
                                }).join('')}
                            </div>
                        ` : ''}
                        ${searchResults.playlists.length > 0 ? `
                            <div class="search-category">
                                <div class="search-category-title">Плейлисты</div>
                                ${searchResults.playlists.map(playlist => `
                                    <div class="search-result-item" data-type="playlist" data-id="${playlist.id}">
                                        <div class="list-item-cover" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); display: flex; align-items: center; justify-content: center; color: white;">
                                            📀
                                        </div>
                                        <div class="list-item-content">
                                            <div class="list-item-title">${Utils.escapeHtml(playlist.title)}</div>
                                            <div class="list-item-subtitle">${playlist.trackCount} треков</div>
                                        </div>
                                    </div>
                                `).join('')}
                            </div>
                        ` : ''}
                    </div>
                ` : ''}
            </div>
        `;
        
        div.addEventListener('click', (e) => {
            if (e.target === div) {
                AppActions.closeSearch();
            }
            
            const resultItem = e.target.closest('.search-result-item');
            if (resultItem) {
                const type = resultItem.dataset.type;
                const id = parseInt(resultItem.dataset.id);
                
                if (type === 'track') {
                    const track = MockData.tracks.find(t => t.id === id);
                    AppActions.playTrack(track);
                } else if (type === 'artist') {
                    Utils.setHashUrl(`/profile/${id}`);
                } else if (type === 'playlist') {
                    Utils.setHashUrl(`/playlist/${id}`);
                }
                
                AppActions.closeSearch();
            }
        });
        
        const searchInput = div.querySelector('#search-input');
        searchInput.addEventListener('input', Utils.debounce((e) => {
            AppActions.search(e.target.value);
        }, 300));
        
        searchInput.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                AppActions.closeSearch();
            }
        });
        
        return div;
    }
    
    onMount() {
        this.unsubscribers.push(
            store.subscribe('isSearchOpen', () => this.update()),
            store.subscribe('searchResults', () => this.update())
        );
    }
}

const router = new Router();

router.addRoute('/', () => {
    if (!store.state.isAuthenticated) {
        return new AuthPage();
    }
    if (!store.state.hasCompletedOnboarding) {
        return new OnboardingPage();
    }
    return new HomePage();
});

router.addRoute('/onboarding', () => new OnboardingPage());
router.addRoute('/profile/:id', (params) => new ProfilePage(params));
router.addRoute('/playlist/:id', (params) => new PlaylistPage(params));
router.addRoute('/chat', () => new ChatPage());
router.addRoute('/liked', () => new LikedPage());
router.addRoute('/history', () => new HistoryPage());
router.addRoute('/radio', () => new RadioPage());
router.addRoute('/player', () => new PlayerPage());
router.addRoute('/admin', () => new AdminPage());

document.addEventListener('DOMContentLoaded', () => {
    Auth.checkAuth(); // Check if user is already logged in
    new Components.PlayerBar().mount(document.body);
    new Components.NotificationToast().mount(document.body);
    new SearchModal().mount(document.body);
    
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            AppActions.openSearch();
        }
    });
});
