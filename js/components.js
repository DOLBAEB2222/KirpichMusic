class Component {
    constructor(props = {}) {
        this.props = props;
        this.el = null;
        this.unsubscribers = [];
    }
    
    render() {
        return document.createElement('div');
    }
    
    mount(parent) {
        this.el = this.render();
        if (typeof parent === 'string') {
            parent = document.querySelector(parent);
        }
        if (parent && this.el) {
            parent.appendChild(this.el);
        }
        this.onMount();
        return this;
    }
    
    onMount() {}
    
    unmount() {
        this.unsubscribers.forEach(unsub => unsub());
        this.unsubscribers = [];
        if (this.el && this.el.parentNode) {
            this.el.parentNode.removeChild(this.el);
        }
        this.onUnmount();
    }
    
    onUnmount() {}
    
    update() {
        if (this.el && this.el.parentNode) {
            const newEl = this.render();
            this.el.parentNode.replaceChild(newEl, this.el);
            this.el = newEl;
        }
    }
}

class Icon extends Component {
    render() {
        const { name, size = 24 } = this.props;
        const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        svg.setAttribute('width', size);
        svg.setAttribute('height', size);
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('fill', 'none');
        svg.setAttribute('stroke', 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('stroke-linecap', 'round');
        svg.setAttribute('stroke-linejoin', 'round');
        
        const icons = {
            play: '<polygon points="5 3 19 12 5 21 5 3"></polygon>',
            pause: '<rect x="6" y="4" width="4" height="16"></rect><rect x="14" y="4" width="4" height="16"></rect>',
            'skip-forward': '<polygon points="5 4 15 12 5 20 5 4"></polygon><line x1="19" y1="5" x2="19" y2="19"></line>',
            'skip-back': '<polygon points="19 20 9 12 19 4 19 20"></polygon><line x1="5" y1="19" x2="5" y2="5"></line>',
            shuffle: '<polyline points="16 3 21 3 21 8"></polyline><line x1="4" y1="20" x2="21" y2="3"></line><polyline points="21 16 21 21 16 21"></polyline><line x1="15" y1="15" x2="21" y2="21"></line><line x1="4" y1="4" x2="9" y2="9"></line>',
            repeat: '<polyline points="17 1 21 5 17 9"></polyline><path d="M3 11V9a4 4 0 0 1 4-4h14"></path><polyline points="7 23 3 19 7 15"></polyline><path d="M21 13v2a4 4 0 0 1-4 4H3"></path>',
            heart: '<path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"></path>',
            'heart-filled': '<path fill="currentColor" d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"></path>',
            'volume-2': '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon><path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"></path>',
            'volume-x': '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"></polygon><line x1="23" y1="9" x2="17" y2="15"></line><line x1="17" y1="9" x2="23" y2="15"></line>',
            'more-horizontal': '<circle cx="12" cy="12" r="1"></circle><circle cx="19" cy="12" r="1"></circle><circle cx="5" cy="12" r="1"></circle>',
            search: '<circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line>',
            home: '<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"></path><polyline points="9 22 9 12 15 12 15 22"></polyline>',
            music: '<path d="M9 18V5l12-2v13"></path><circle cx="6" cy="18" r="3"></circle><circle cx="18" cy="16" r="3"></circle>',
            user: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle>',
            list: '<line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line><line x1="3" y1="6" x2="3.01" y2="6"></line><line x1="3" y1="12" x2="3.01" y2="12"></line><line x1="3" y1="18" x2="3.01" y2="18"></line>',
            clock: '<circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline>',
            'message-circle': '<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"></path>',
            radio: '<circle cx="12" cy="12" r="2"></circle><path d="M16.24 7.76a6 6 0 0 1 0 8.49m-8.48-.01a6 6 0 0 1 0-8.49m11.31-2.82a10 10 0 0 1 0 14.14m-14.14 0a10 10 0 0 1 0-14.14"></path>',
            plus: '<line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line>',
            x: '<line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line>',
            check: '<polyline points="20 6 9 17 4 12"></polyline>',
            'chevron-down': '<polyline points="6 9 12 15 18 9"></polyline>',
            'chevron-up': '<polyline points="18 15 12 9 6 15"></polyline>',
            'chevron-right': '<polyline points="9 18 15 12 9 6"></polyline>',
            'chevron-left': '<polyline points="15 18 9 12 15 6"></polyline>',
            share: '<circle cx="18" cy="5" r="3"></circle><circle cx="6" cy="12" r="3"></circle><circle cx="18" cy="19" r="3"></circle><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"></line><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"></line>',
            settings: '<circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>',
            'maximize': '<path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"></path>',
            menu: '<line x1="3" y1="12" x2="21" y2="12"></line><line x1="3" y1="6" x2="21" y2="6"></line><line x1="3" y1="18" x2="21" y2="18"></line>',
            'log-out': '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path><polyline points="16 17 21 12 16 7"></polyline><line x1="21" y1="12" x2="9" y2="12"></line>',
            bell: '<path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path><path d="M13.73 21a2 2 0 0 1-3.46 0"></path>'
        };
        
        svg.innerHTML = icons[name] || icons.music;
        return svg;
    }
}

class TrackCard extends Component {
    render() {
        const { track } = this.props;
        const div = document.createElement('div');
        div.className = 'card';
        
        const isLiked = AppActions.isLiked(track.id);
        
        div.innerHTML = `
            <div class="card-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 3rem;">
                🎵
            </div>
            <div class="card-body">
                <div class="card-title truncate">${Utils.escapeHtml(track.title)}</div>
                <div class="card-subtitle truncate">${Utils.escapeHtml(track.artist)}</div>
            </div>
        `;
        
        div.addEventListener('click', () => {
            AppActions.playTrack(track, [track], 0);
        });
        
        return div;
    }
}

class ArtistCard extends Component {
    render() {
        const { artist } = this.props;
        const div = document.createElement('div');
        div.className = 'card';
        
        const initials = Utils.getInitials(artist.displayName);
        const isFollowing = AppActions.isFollowing(artist.id);
        
        div.innerHTML = `
            <div class="card-cover" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 2rem; font-weight: 700;">
                ${initials}
            </div>
            <div class="card-body">
                <div class="card-title truncate">
                    ${Utils.escapeHtml(artist.displayName)}
                    ${artist.verified ? '<span style="color: var(--color-accent-primary);">✓</span>' : ''}
                </div>
                <div class="card-subtitle">Артист</div>
            </div>
        `;
        
        div.addEventListener('click', () => {
            Utils.setHashUrl(`/profile/${artist.id}`);
        });
        
        return div;
    }
}

class PlaylistCard extends Component {
    render() {
        const { playlist } = this.props;
        const div = document.createElement('div');
        div.className = 'card';
        
        div.innerHTML = `
            <div class="card-cover" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 3rem;">
                📀
            </div>
            <div class="card-body">
                <div class="card-title truncate">${Utils.escapeHtml(playlist.title)}</div>
                <div class="card-subtitle truncate">${playlist.trackCount} треков</div>
            </div>
        `;
        
        div.addEventListener('click', () => {
            Utils.setHashUrl(`/playlist/${playlist.id}`);
        });
        
        return div;
    }
}

class Sidebar extends Component {
    render() {
        const div = document.createElement('div');
        div.className = 'sidebar';
        
        const navItems = [
            { icon: 'home', label: 'Главная', path: '/' },
            { icon: 'search', label: 'Поиск', action: 'search' },
            { icon: 'heart', label: 'Понравившиеся', path: '/liked' },
            { icon: 'clock', label: 'История', path: '/history' },
            { icon: 'message-circle', label: 'IRC Чат', path: '/chat' },
            { icon: 'radio', label: 'Радио', path: '/radio' }
        ];
        
        const playlistItems = [
            ...MockData.playlists.slice(0, 5),
            ...store.state.userPlaylists
        ];
        
        div.innerHTML = `
            <div class="sidebar-header">
                <div class="sidebar-logo">
                    🧱 KirpichMusic
                </div>
            </div>
            <div class="sidebar-nav">
                <div class="sidebar-nav-section">
                    ${navItems.map(item => `
                        <a href="#${item.path || ''}" class="sidebar-nav-item" data-action="${item.action || ''}">
                            ${new Icon({ name: item.icon, size: 20 }).render().outerHTML}
                            <span>${item.label}</span>
                        </a>
                    `).join('')}
                </div>
                <div class="sidebar-nav-section">
                    <div class="sidebar-nav-title">Плейлисты</div>
                    ${playlistItems.map(playlist => `
                        <a href="#/playlist/${playlist.id}" class="sidebar-nav-item">
                            <span>📀</span>
                            <span class="truncate">${Utils.escapeHtml(playlist.title)}</span>
                        </a>
                    `).join('')}
                    <button class="sidebar-nav-item" data-action="create-playlist" style="border: none; background: none; width: 100%; text-align: left; cursor: pointer;">
                        ${new Icon({ name: 'plus', size: 20 }).render().outerHTML}
                        <span>Создать плейлист</span>
                    </button>
                </div>
            </div>
        `;
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (target) {
                e.preventDefault();
                const action = target.dataset.action;
                if (action === 'search') {
                    AppActions.openSearch();
                } else if (action === 'create-playlist') {
                    this.createPlaylist();
                }
            }
        });
        
        return div;
    }
    
    createPlaylist() {
        const name = prompt('Название плейлиста:');
        if (name) {
            const playlist = AppActions.createPlaylist(name);
            Utils.setHashUrl(`/playlist/${playlist.id}`);
        }
    }
}

class PlayerBar extends Component {
    onMount() {
        this.unsubscribers.push(
            store.subscribe('currentTrack', () => this.update()),
            store.subscribe('isPlaying', () => this.update()),
            store.subscribe('volume', () => this.updateVolume()),
            store.subscribe('isMuted', () => this.updateVolume()),
            store.subscribe('repeat', () => this.update()),
            store.subscribe('shuffle', () => this.update())
        );
    }
    
    render() {
        const { currentTrack, isPlaying, volume, isMuted, repeat, shuffle } = store.state;
        
        if (!currentTrack) {
            return document.createElement('div');
        }
        
        const div = document.createElement('div');
        div.className = 'player-bar';
        
        const isLiked = AppActions.isLiked(currentTrack.id);
        
        div.innerHTML = `
            <div class="player-track-info">
                <div class="player-cover" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 2rem;">
                    🎵
                </div>
                <div class="player-details">
                    <div class="player-title truncate">${Utils.escapeHtml(currentTrack.title)}</div>
                    <div class="player-artist truncate">${Utils.escapeHtml(currentTrack.artist)}</div>
                </div>
                <button class="btn btn-icon btn-ghost player-like">
                    ${new Icon({ name: isLiked ? 'heart-filled' : 'heart', size: 20 }).render().outerHTML}
                </button>
            </div>
            
            <div class="player-controls">
                <div class="player-buttons">
                    <button class="btn btn-icon btn-ghost player-btn" data-action="shuffle">
                        ${new Icon({ name: 'shuffle', size: 20 }).render().outerHTML}
                    </button>
                    <button class="btn btn-icon btn-ghost player-btn" data-action="previous">
                        ${new Icon({ name: 'skip-back', size: 20 }).render().outerHTML}
                    </button>
                    <button class="btn btn-icon player-btn player-btn-play" data-action="play">
                        ${new Icon({ name: isPlaying ? 'pause' : 'play', size: 24 }).render().outerHTML}
                    </button>
                    <button class="btn btn-icon btn-ghost player-btn" data-action="next">
                        ${new Icon({ name: 'skip-forward', size: 20 }).render().outerHTML}
                    </button>
                    <button class="btn btn-icon btn-ghost player-btn" data-action="repeat">
                        ${new Icon({ name: 'repeat', size: 20 }).render().outerHTML}
                    </button>
                </div>
                <div class="player-timeline">
                    <span class="player-time">0:00</span>
                    <div class="progress">
                        <div class="progress-bar" style="width: 0%"></div>
                    </div>
                    <span class="player-time">${Utils.formatTime(currentTrack.duration)}</span>
                </div>
            </div>
            
            <div class="player-extras">
                <button class="btn btn-icon btn-ghost" data-action="expand">
                    ${new Icon({ name: 'maximize', size: 20 }).render().outerHTML}
                </button>
                <div class="player-volume">
                    <button class="btn btn-icon btn-ghost" data-action="mute">
                        ${new Icon({ name: isMuted ? 'volume-x' : 'volume-2', size: 20 }).render().outerHTML}
                    </button>
                    <div class="progress">
                        <div class="progress-bar" style="width: ${isMuted ? 0 : volume * 100}%"></div>
                    </div>
                </div>
                <button class="btn btn-icon btn-ghost" data-action="queue">
                    ${new Icon({ name: 'list', size: 20 }).render().outerHTML}
                </button>
            </div>
        `;
        
        div.addEventListener('click', (e) => {
            const target = e.target.closest('[data-action]');
            if (!target) return;
            
            const action = target.dataset.action;
            switch (action) {
                case 'play':
                    AppActions.togglePlay();
                    break;
                case 'previous':
                    AppActions.previousTrack();
                    break;
                case 'next':
                    AppActions.nextTrack();
                    break;
                case 'shuffle':
                    AppActions.toggleShuffle();
                    break;
                case 'repeat':
                    const modes = ['off', 'all', 'one'];
                    const currentIndex = modes.indexOf(repeat);
                    AppActions.setRepeat(modes[(currentIndex + 1) % modes.length]);
                    break;
                case 'mute':
                    AppActions.toggleMute();
                    break;
                case 'expand':
                    AppActions.togglePlayerExpanded();
                    Utils.setHashUrl('/player');
                    break;
                case 'queue':
                    Utils.setHashUrl('/player');
                    break;
            }
        });
        
        const likeBtn = div.querySelector('.player-like');
        likeBtn.addEventListener('click', () => {
            AppActions.toggleLike(currentTrack);
        });
        
        return div;
    }
    
    updateVolume() {
        if (this.el) {
            const volumeBar = this.el.querySelector('.player-volume .progress-bar');
            if (volumeBar) {
                const { volume, isMuted } = store.state;
                volumeBar.style.width = `${isMuted ? 0 : volume * 100}%`;
            }
        }
    }
}

class NotificationToast extends Component {
    render() {
        const { notifications } = store.state;
        
        const div = document.createElement('div');
        div.className = 'toast-container';
        
        notifications.forEach(notif => {
            const toast = document.createElement('div');
            toast.className = 'notification';
            toast.innerHTML = `
                <div class="notification-icon">
                    ${new Icon({ name: 'bell', size: 24 }).render().outerHTML}
                </div>
                <div class="notification-content">
                    ${notif.title ? `<div class="notification-title">${Utils.escapeHtml(notif.title)}</div>` : ''}
                    ${notif.message ? `<div class="notification-message">${Utils.escapeHtml(notif.message)}</div>` : ''}
                </div>
                <button class="btn btn-icon btn-ghost notification-close" data-id="${notif.id}">
                    ${new Icon({ name: 'x', size: 20 }).render().outerHTML}
                </button>
            `;
            
            const closeBtn = toast.querySelector('.notification-close');
            closeBtn.addEventListener('click', () => {
                AppActions.removeNotification(notif.id);
            });
            
            div.appendChild(toast);
        });
        
        return div;
    }
    
    onMount() {
        this.unsubscribers.push(
            store.subscribe('notifications', () => this.update())
        );
    }
}

window.Components = {
    Component,
    Icon,
    TrackCard,
    ArtistCard,
    PlaylistCard,
    Sidebar,
    PlayerBar,
    NotificationToast
};
