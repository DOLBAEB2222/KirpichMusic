class Auth {
    static async login(username, password) {
        // Mock API call
        return new Promise((resolve, reject) => {
            setTimeout(() => {
                if (username === 'admin' && password === 'admin') {
                    const user = {
                        id: 1,
                        username: 'admin',
                        displayName: 'Администратор',
                        role: 'admin',
                        avatar: null,
                        type: 'artist'
                    };
                    const token = 'mock-jwt-token-admin';
                    this.setSession(user, token);
                    resolve(user);
                } else if (username && password) {
                    const user = {
                        id: 2,
                        username: username,
                        displayName: username,
                        role: 'user',
                        avatar: null,
                        type: 'listener'
                    };
                    const token = 'mock-jwt-token-user';
                    this.setSession(user, token);
                    resolve(user);
                } else {
                    reject(new Error('Invalid credentials'));
                }
            }, 500);
        });
    }

    static logout() {
        localStorage.removeItem('ks_user');
        localStorage.removeItem('ks_token');
        store.setState({ isAuthenticated: false, currentUser: null });
        Utils.setHashUrl('/');
    }

    static setSession(user, token) {
        localStorage.setItem('ks_user', JSON.stringify(user));
        localStorage.setItem('ks_token', token);
        store.setState({ isAuthenticated: true, currentUser: user });
    }

    static checkAuth() {
        const user = localStorage.getItem('ks_user');
        const token = localStorage.getItem('ks_token');
        if (user && token) {
            store.setState({ isAuthenticated: true, currentUser: JSON.parse(user) });
            return true;
        }
        return false;
    }

    static isAdmin() {
        return store.state.currentUser?.role === 'admin';
    }
}

window.Auth = Auth;
