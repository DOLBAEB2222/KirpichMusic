const createStore = (initialState) => {
    const listeners = new Map();
    
    const notify = (path) => {
        const pathListeners = listeners.get(path) || [];
        pathListeners.forEach(listener => listener(state));
        
        const allListeners = listeners.get('*') || [];
        allListeners.forEach(listener => listener(state));
    };
    
    const handler = {
        set(target, property, value) {
            const oldValue = target[property];
            if (oldValue !== value) {
                target[property] = value;
                notify(property);
            }
            return true;
        }
    };
    
    const state = new Proxy(initialState, handler);
    
    return {
        get state() {
            return state;
        },
        
        subscribe(path, listener) {
            if (!listeners.has(path)) {
                listeners.set(path, []);
            }
            listeners.get(path).push(listener);
            
            return () => {
                const pathListeners = listeners.get(path);
                const index = pathListeners.indexOf(listener);
                if (index > -1) {
                    pathListeners.splice(index, 1);
                }
            };
        },
        
        setState(updates) {
            Object.entries(updates).forEach(([key, value]) => {
                state[key] = value;
            });
        }
    };
};

const store = createStore({
    currentUser: Utils.storage.get('currentUser', null),
    isAuthenticated: Utils.storage.get('isAuthenticated', false),
    hasCompletedOnboarding: Utils.storage.get('hasCompletedOnboarding', false),
    
    currentTrack: null,
    isPlaying: false,
    currentTime: 0,
    duration: 0,
    volume: Utils.storage.get('volume', 0.7),
    isMuted: false,
    repeat: Utils.storage.get('repeat', 'off'),
    shuffle: Utils.storage.get('shuffle', false),
    
    queue: Utils.storage.get('queue', []),
    queueIndex: Utils.storage.get('queueIndex', 0),
    originalQueue: [],
    
    likedTracks: Utils.storage.get('likedTracks', []),
    likedAlbums: Utils.storage.get('likedAlbums', []),
    followedArtists: Utils.storage.get('followedArtists', []),
    userPlaylists: Utils.storage.get('userPlaylists', []),
    
    history: Utils.storage.get('history', []),
    
    currentIrcChannel: '#general',
    unreadMessages: Utils.storage.get('unreadMessages', {}),
    
    notifications: [],
    
    isPlayerExpanded: false,
    
    searchQuery: '',
    searchResults: null,
    isSearchOpen: false
});

store.subscribe('currentUser', () => {
    Utils.storage.set('currentUser', store.state.currentUser);
});

store.subscribe('isAuthenticated', () => {
    Utils.storage.set('isAuthenticated', store.state.isAuthenticated);
});

store.subscribe('hasCompletedOnboarding', () => {
    Utils.storage.set('hasCompletedOnboarding', store.state.hasCompletedOnboarding);
});

store.subscribe('volume', () => {
    Utils.storage.set('volume', store.state.volume);
});

store.subscribe('repeat', () => {
    Utils.storage.set('repeat', store.state.repeat);
});

store.subscribe('shuffle', () => {
    Utils.storage.set('shuffle', store.state.shuffle);
});

store.subscribe('queue', () => {
    Utils.storage.set('queue', store.state.queue);
});

store.subscribe('queueIndex', () => {
    Utils.storage.set('queueIndex', store.state.queueIndex);
});

store.subscribe('likedTracks', () => {
    Utils.storage.set('likedTracks', store.state.likedTracks);
});

store.subscribe('likedAlbums', () => {
    Utils.storage.set('likedAlbums', store.state.likedAlbums);
});

store.subscribe('followedArtists', () => {
    Utils.storage.set('followedArtists', store.state.followedArtists);
});

store.subscribe('userPlaylists', () => {
    Utils.storage.set('userPlaylists', store.state.userPlaylists);
});

store.subscribe('history', () => {
    Utils.storage.set('history', store.state.history);
});

store.subscribe('unreadMessages', () => {
    Utils.storage.set('unreadMessages', store.state.unreadMessages);
});

const AppActions = {
    login(user) {
        store.setState({
            currentUser: user,
            isAuthenticated: true
        });
    },
    
    logout() {
        store.setState({
            currentUser: null,
            isAuthenticated: false,
            hasCompletedOnboarding: false,
            currentTrack: null,
            isPlaying: false,
            queue: [],
            queueIndex: 0
        });
        Utils.storage.clear();
    },
    
    loginAsGuest() {
        const guestUser = {
            id: 999,
            username: 'guest',
            displayName: 'Гость',
            avatar: null,
            bio: '',
            verified: false,
            type: 'listener',
            followers: 0,
            following: 0,
            tracks: 0,
            playlists: 0
        };
        this.login(guestUser);
    },
    
    completeOnboarding() {
        store.setState({ hasCompletedOnboarding: true });
    },
    
    playTrack(track, queue = null, queueIndex = 0) {
        if (queue) {
            store.setState({
                currentTrack: track,
                isPlaying: true,
                queue: queue,
                queueIndex: queueIndex,
                originalQueue: queue
            });
        } else {
            store.setState({
                currentTrack: track,
                isPlaying: true
            });
        }
        
        if (track && !store.state.history.find(h => h.id === track.id)) {
            const history = [
                { ...track, playedAt: new Date().toISOString() },
                ...store.state.history
            ].slice(0, 100);
            store.setState({ history });
        }
    },
    
    togglePlay() {
        store.setState({ isPlaying: !store.state.isPlaying });
    },
    
    pause() {
        store.setState({ isPlaying: false });
    },
    
    nextTrack() {
        const { queue, queueIndex } = store.state;
        if (queue.length === 0) return;
        
        const nextIndex = (queueIndex + 1) % queue.length;
        const nextTrack = queue[nextIndex];
        
        store.setState({
            currentTrack: nextTrack,
            queueIndex: nextIndex,
            isPlaying: true
        });
    },
    
    previousTrack() {
        const { queue, queueIndex } = store.state;
        if (queue.length === 0) return;
        
        const prevIndex = queueIndex === 0 ? queue.length - 1 : queueIndex - 1;
        const prevTrack = queue[prevIndex];
        
        store.setState({
            currentTrack: prevTrack,
            queueIndex: prevIndex,
            isPlaying: true
        });
    },
    
    setVolume(volume) {
        store.setState({ 
            volume: Utils.clamp(volume, 0, 1),
            isMuted: false 
        });
    },
    
    toggleMute() {
        store.setState({ isMuted: !store.state.isMuted });
    },
    
    setRepeat(mode) {
        store.setState({ repeat: mode });
    },
    
    toggleShuffle() {
        const shuffle = !store.state.shuffle;
        store.setState({ shuffle });
        
        if (shuffle) {
            const { currentTrack, queue } = store.state;
            const currentIndex = queue.findIndex(t => t.id === currentTrack?.id);
            const withoutCurrent = queue.filter((_, i) => i !== currentIndex);
            const shuffled = Utils.shuffle(withoutCurrent);
            const newQueue = currentTrack ? [currentTrack, ...shuffled] : shuffled;
            store.setState({ 
                queue: newQueue,
                queueIndex: currentTrack ? 0 : -1
            });
        } else {
            store.setState({ queue: store.state.originalQueue });
        }
    },
    
    addToQueue(tracks) {
        const tracksArray = Array.isArray(tracks) ? tracks : [tracks];
        store.setState({ 
            queue: [...store.state.queue, ...tracksArray]
        });
        this.showNotification({
            title: 'Добавлено в очередь',
            message: `${tracksArray.length} ${tracksArray.length === 1 ? 'трек' : 'треков'}`
        });
    },
    
    removeFromQueue(index) {
        const queue = [...store.state.queue];
        queue.splice(index, 1);
        store.setState({ queue });
    },
    
    reorderQueue(fromIndex, toIndex) {
        const queue = [...store.state.queue];
        const [removed] = queue.splice(fromIndex, 1);
        queue.splice(toIndex, 0, removed);
        store.setState({ queue });
    },
    
    toggleLike(track) {
        const likedTracks = [...store.state.likedTracks];
        const index = likedTracks.findIndex(t => t.id === track.id);
        
        if (index > -1) {
            likedTracks.splice(index, 1);
            this.showNotification({
                title: 'Удалено из понравившихся',
                message: track.title
            });
        } else {
            likedTracks.push(track);
            this.showNotification({
                title: 'Добавлено в понравившиеся',
                message: track.title
            });
        }
        
        store.setState({ likedTracks });
    },
    
    isLiked(trackId) {
        return store.state.likedTracks.some(t => t.id === trackId);
    },
    
    toggleFollowArtist(artistId) {
        const followedArtists = [...store.state.followedArtists];
        const index = followedArtists.indexOf(artistId);
        
        if (index > -1) {
            followedArtists.splice(index, 1);
        } else {
            followedArtists.push(artistId);
        }
        
        store.setState({ followedArtists });
    },
    
    isFollowing(artistId) {
        return store.state.followedArtists.includes(artistId);
    },
    
    createPlaylist(name, description = '') {
        const playlist = {
            id: Utils.generateId(),
            title: name,
            description,
            cover: null,
            owner: store.state.currentUser?.displayName || 'Неизвестный',
            ownerId: store.state.currentUser?.id || 0,
            trackCount: 0,
            duration: 0,
            isPublic: true,
            collaborative: false,
            tracks: [],
            createdAt: new Date().toISOString()
        };
        
        const userPlaylists = [...store.state.userPlaylists, playlist];
        store.setState({ userPlaylists });
        
        this.showNotification({
            title: 'Плейлист создан',
            message: name
        });
        
        return playlist;
    },
    
    addToPlaylist(playlistId, track) {
        const userPlaylists = [...store.state.userPlaylists];
        const playlist = userPlaylists.find(p => p.id === playlistId);
        
        if (playlist && !playlist.tracks.includes(track.id)) {
            playlist.tracks.push(track.id);
            playlist.trackCount = playlist.tracks.length;
            store.setState({ userPlaylists });
            
            this.showNotification({
                title: 'Добавлено в плейлист',
                message: `${track.title} → ${playlist.title}`
            });
        }
    },
    
    removeFromPlaylist(playlistId, trackId) {
        const userPlaylists = [...store.state.userPlaylists];
        const playlist = userPlaylists.find(p => p.id === playlistId);
        
        if (playlist) {
            playlist.tracks = playlist.tracks.filter(id => id !== trackId);
            playlist.trackCount = playlist.tracks.length;
            store.setState({ userPlaylists });
        }
    },
    
    deletePlaylist(playlistId) {
        const userPlaylists = store.state.userPlaylists.filter(p => p.id !== playlistId);
        store.setState({ userPlaylists });
        
        this.showNotification({
            title: 'Плейлист удалён'
        });
    },
    
    setIrcChannel(channel) {
        const unreadMessages = { ...store.state.unreadMessages };
        delete unreadMessages[channel];
        store.setState({ 
            currentIrcChannel: channel,
            unreadMessages
        });
    },
    
    addIrcMessage(message) {
        MockData.ircMessages.push(message);
        
        if (message.channel !== store.state.currentIrcChannel) {
            const unreadMessages = { ...store.state.unreadMessages };
            unreadMessages[message.channel] = (unreadMessages[message.channel] || 0) + 1;
            store.setState({ unreadMessages });
        }
    },
    
    showNotification(notification) {
        const id = Utils.generateId();
        const notif = {
            id,
            ...notification,
            timestamp: Date.now()
        };
        
        store.setState({
            notifications: [...store.state.notifications, notif]
        });
        
        setTimeout(() => {
            this.removeNotification(id);
        }, 5000);
    },
    
    removeNotification(id) {
        store.setState({
            notifications: store.state.notifications.filter(n => n.id !== id)
        });
    },
    
    togglePlayerExpanded() {
        store.setState({ isPlayerExpanded: !store.state.isPlayerExpanded });
    },
    
    openSearch() {
        store.setState({ isSearchOpen: true });
    },
    
    closeSearch() {
        store.setState({ isSearchOpen: false, searchQuery: '', searchResults: null });
    },
    
    search(query) {
        store.setState({ searchQuery: query });
        
        if (!query.trim()) {
            store.setState({ searchResults: null });
            return;
        }
        
        const lowerQuery = query.toLowerCase();
        
        const tracks = MockData.tracks.filter(t => 
            t.title.toLowerCase().includes(lowerQuery) ||
            t.artist.toLowerCase().includes(lowerQuery) ||
            t.album.toLowerCase().includes(lowerQuery)
        ).slice(0, 5);
        
        const artists = MockData.users.filter(u => 
            u.type === 'artist' &&
            (u.displayName.toLowerCase().includes(lowerQuery) ||
             u.username.toLowerCase().includes(lowerQuery))
        ).slice(0, 5);
        
        const playlists = [...MockData.playlists, ...store.state.userPlaylists].filter(p =>
            p.title.toLowerCase().includes(lowerQuery) ||
            p.description.toLowerCase().includes(lowerQuery)
        ).slice(0, 5);
        
        store.setState({
            searchResults: { tracks, artists, playlists }
        });
    }
};

window.store = store;
window.AppActions = AppActions;
