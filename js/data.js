const MockData = {
    users: [
        {
            id: 1,
            username: 'kirpich_master',
            displayName: 'Кирпич Мастер',
            avatar: null,
            bio: 'Основатель KirpichMusic. Создаю музыку на стыке индастриала и электроники.',
            verified: true,
            type: 'artist',
            followers: 125430,
            following: 234,
            tracks: 42,
            playlists: 8
        },
        {
            id: 2,
            username: 'electro_wave',
            displayName: 'Электро Волна',
            avatar: null,
            bio: 'Экспериментальная электроника и синтвейв',
            verified: true,
            type: 'artist',
            followers: 89320,
            following: 156,
            tracks: 31,
            playlists: 5
        },
        {
            id: 3,
            username: 'dark_rhythm',
            displayName: 'Тёмный Ритм',
            avatar: null,
            bio: 'Dark techno & industrial beats',
            verified: true,
            type: 'artist',
            followers: 67890,
            following: 98,
            tracks: 28,
            playlists: 4
        },
        {
            id: 4,
            username: 'neon_dreams',
            displayName: 'Неоновые Сны',
            avatar: null,
            bio: 'Синтвейв и ретрофутуризм',
            verified: false,
            type: 'artist',
            followers: 45678,
            following: 145,
            tracks: 19,
            playlists: 3
        },
        {
            id: 5,
            username: 'heavy_bass',
            displayName: 'Тяжёлый Бас',
            avatar: null,
            bio: 'Bassline культура',
            verified: true,
            type: 'artist',
            followers: 34567,
            following: 201,
            tracks: 25,
            playlists: 6
        },
        {
            id: 6,
            username: 'analog_soul',
            displayName: 'Аналоговая Душа',
            avatar: null,
            bio: 'Винтажные синтезаторы и тёплый звук',
            verified: false,
            type: 'artist',
            followers: 23456,
            following: 178,
            tracks: 15,
            playlists: 2
        },
        {
            id: 7,
            username: 'cyber_punk',
            displayName: 'Киберпанк',
            avatar: null,
            bio: 'Музыка будущего, которое уже наступило',
            verified: true,
            type: 'artist',
            followers: 91234,
            following: 87,
            tracks: 38,
            playlists: 7
        },
        {
            id: 8,
            username: 'ambient_space',
            displayName: 'Космический Эмбиент',
            avatar: null,
            bio: 'Путешествия в глубины космоса через звук',
            verified: false,
            type: 'artist',
            followers: 18765,
            following: 134,
            tracks: 22,
            playlists: 4
        },
        {
            id: 9,
            username: 'listener_alex',
            displayName: 'Алексей',
            avatar: null,
            bio: 'Меломан и коллекционер винила',
            verified: false,
            type: 'listener',
            followers: 234,
            following: 456,
            tracks: 0,
            playlists: 12
        },
        {
            id: 10,
            username: 'music_lover_maria',
            displayName: 'Мария',
            avatar: null,
            bio: 'Люблю электронику и индастриал',
            verified: false,
            type: 'listener',
            followers: 156,
            following: 289,
            tracks: 0,
            playlists: 8
        },
        {
            id: 11,
            username: 'industrial_fan',
            displayName: 'Дмитрий',
            avatar: null,
            bio: 'Индустриальная музыка - это жизнь',
            verified: false,
            type: 'listener',
            followers: 98,
            following: 312,
            tracks: 0,
            playlists: 15
        },
        {
            id: 12,
            username: 'electronic_girl',
            displayName: 'Катя',
            avatar: null,
            bio: 'Танцую под электронику',
            verified: false,
            type: 'listener',
            followers: 445,
            following: 123,
            tracks: 0,
            playlists: 6
        },
        {
            id: 13,
            username: 'synth_collector',
            displayName: 'Сергей',
            avatar: null,
            bio: 'Коллекционирую синтезаторы и пластинки',
            verified: false,
            type: 'listener',
            followers: 567,
            following: 234,
            tracks: 0,
            playlists: 20
        },
        {
            id: 14,
            username: 'night_rider',
            displayName: 'Ночной Гонщик',
            avatar: null,
            bio: 'Ночные поездки под синтвейв',
            verified: false,
            type: 'listener',
            followers: 289,
            following: 167,
            tracks: 0,
            playlists: 9
        },
        {
            id: 15,
            username: 'bass_head',
            displayName: 'Павел',
            avatar: null,
            bio: 'Если нет баса - не музыка',
            verified: false,
            type: 'listener',
            followers: 178,
            following: 456,
            tracks: 0,
            playlists: 11
        }
    ],

    tracks: [
        { id: 1, title: 'Индустриальный рассвет', artist: 'Кирпич Мастер', artistId: 1, album: 'Заводские ритмы', duration: 245, plays: 1234567, likes: 45678, cover: null, genre: 'Industrial' },
        { id: 2, title: 'Металлический пульс', artist: 'Кирпич Мастер', artistId: 1, album: 'Заводские ритмы', duration: 198, plays: 987654, likes: 34567, cover: null, genre: 'Industrial' },
        { id: 3, title: 'Электрические мечты', artist: 'Электро Волна', artistId: 2, album: 'Синтез будущего', duration: 312, plays: 876543, likes: 56789, cover: null, genre: 'Electronica' },
        { id: 4, title: 'Неоновый дождь', artist: 'Электро Волна', artistId: 2, album: 'Синтез будущего', duration: 267, plays: 765432, likes: 43210, cover: null, genre: 'Electronica' },
        { id: 5, title: 'Тёмная материя', artist: 'Тёмный Ритм', artistId: 3, album: 'Глубина техно', duration: 423, plays: 654321, likes: 38901, cover: null, genre: 'Techno' },
        { id: 6, title: 'Бетонные джунгли', artist: 'Тёмный Ритм', artistId: 3, album: 'Глубина техно', duration: 356, plays: 543210, likes: 29012, cover: null, genre: 'Techno' },
        { id: 7, title: 'Ретро волна', artist: 'Неоновые Сны', artistId: 4, album: '80-е возвращаются', duration: 234, plays: 432109, likes: 27890, cover: null, genre: 'Synthwave' },
        { id: 8, title: 'Закат над городом', artist: 'Неоновые Сны', artistId: 4, album: '80-е возвращаются', duration: 289, plays: 398765, likes: 25678, cover: null, genre: 'Synthwave' },
        { id: 9, title: 'Суб-басс атака', artist: 'Тяжёлый Бас', artistId: 5, album: 'Низкие частоты', duration: 278, plays: 567890, likes: 41234, cover: null, genre: 'Bass' },
        { id: 10, title: 'Землетрясение', artist: 'Тяжёлый Бас', artistId: 5, album: 'Низкие частоты', duration: 301, plays: 498765, likes: 36789, cover: null, genre: 'Bass' },
        { id: 11, title: 'Аналоговое тепло', artist: 'Аналоговая Душа', artistId: 6, album: 'Винтажные звуки', duration: 245, plays: 234567, likes: 18901, cover: null, genre: 'Ambient' },
        { id: 12, title: 'Ламповый драйв', artist: 'Аналоговая Душа', artistId: 6, album: 'Винтажные звуки', duration: 267, plays: 198765, likes: 15678, cover: null, genre: 'Ambient' },
        { id: 13, title: 'Нейронные сети', artist: 'Киберпанк', artistId: 7, album: 'Цифровая антиутопия', duration: 334, plays: 987654, likes: 67890, cover: null, genre: 'Cyberpunk' },
        { id: 14, title: 'Голограммы ночи', artist: 'Киберпанк', artistId: 7, album: 'Цифровая антиутопия', duration: 298, plays: 876543, likes: 59012, cover: null, genre: 'Cyberpunk' },
        { id: 15, title: 'Межзвёздное путешествие', artist: 'Космический Эмбиент', artistId: 8, album: 'Бесконечность', duration: 456, plays: 345678, likes: 23456, cover: null, genre: 'Ambient' },
        { id: 16, title: 'Туманность Андромеды', artist: 'Космический Эмбиент', artistId: 8, album: 'Бесконечность', duration: 512, plays: 298765, likes: 19876, cover: null, genre: 'Ambient' },
        { id: 17, title: 'Механический оркестр', artist: 'Кирпич Мастер', artistId: 1, album: 'Заводские ритмы', duration: 289, plays: 765432, likes: 42345, cover: null, genre: 'Industrial' },
        { id: 18, title: 'Стальные нервы', artist: 'Кирпич Мастер', artistId: 1, album: 'Заводские ритмы', duration: 223, plays: 654321, likes: 38765, cover: null, genre: 'Industrial' },
        { id: 19, title: 'Цифровой шторм', artist: 'Электро Волна', artistId: 2, album: 'Синтез будущего', duration: 345, plays: 543210, likes: 34567, cover: null, genre: 'Electronica' },
        { id: 20, title: 'Квантовый скачок', artist: 'Электро Волна', artistId: 2, album: 'Синтез будущего', duration: 278, plays: 456789, likes: 29876, cover: null, genre: 'Electronica' },
        { id: 21, title: 'Подземелье звуков', artist: 'Тёмный Ритм', artistId: 3, album: 'Глубина техно', duration: 398, plays: 398765, likes: 27654, cover: null, genre: 'Techno' },
        { id: 22, title: 'Индустриальный коллапс', artist: 'Тёмный Ритм', artistId: 3, album: 'Глубина техно', duration: 412, plays: 345678, likes: 24567, cover: null, genre: 'Techno' },
        { id: 23, title: 'Полночный круиз', artist: 'Неоновые Сны', artistId: 4, album: '80-е возвращаются', duration: 256, plays: 298765, likes: 22345, cover: null, genre: 'Synthwave' },
        { id: 24, title: 'Розовый закат', artist: 'Неоновые Сны', artistId: 4, album: '80-е возвращаются', duration: 234, plays: 267890, likes: 19876, cover: null, genre: 'Synthwave' },
        { id: 25, title: 'Глубокий грув', artist: 'Тяжёлый Бас', artistId: 5, album: 'Низкие частоты', duration: 312, plays: 456789, likes: 35678, cover: null, genre: 'Bass' },
        { id: 26, title: 'Резонанс', artist: 'Тяжёлый Бас', artistId: 5, album: 'Низкие частоты', duration: 289, plays: 389765, likes: 31234, cover: null, genre: 'Bass' },
        { id: 27, title: 'Осенний меланхолия', artist: 'Аналоговая Душа', artistId: 6, album: 'Винтажные звуки', duration: 298, plays: 234567, likes: 17890, cover: null, genre: 'Ambient' },
        { id: 28, title: 'Эхо прошлого', artist: 'Аналоговая Душа', artistId: 6, album: 'Винтажные звуки', duration: 276, plays: 198765, likes: 14567, cover: null, genre: 'Ambient' },
        { id: 29, title: 'Город будущего', artist: 'Киберпанк', artistId: 7, album: 'Цифровая антиутопия', duration: 367, plays: 876543, likes: 61234, cover: null, genre: 'Cyberpunk' },
        { id: 30, title: 'Электронные грёзы', artist: 'Киберпанк', artistId: 7, album: 'Цифровая антиутопия', duration: 289, plays: 765432, likes: 54321, cover: null, genre: 'Cyberpunk' },
        { id: 31, title: 'Звёздная пыль', artist: 'Космический Эмбиент', artistId: 8, album: 'Бесконечность', duration: 489, plays: 298765, likes: 21098, cover: null, genre: 'Ambient' },
        { id: 32, title: 'Космическая тишина', artist: 'Космический Эмбиент', artistId: 8, album: 'Бесконечность', duration: 534, plays: 267890, likes: 18765, cover: null, genre: 'Ambient' },
        { id: 33, title: 'Заводная мелодия', artist: 'Кирпич Мастер', artistId: 1, album: 'Заводские ритмы', duration: 234, plays: 598765, likes: 39876, cover: null, genre: 'Industrial' },
        { id: 34, title: 'Железная воля', artist: 'Кирпич Мастер', artistId: 1, album: 'Заводские ритмы', duration: 256, plays: 498765, likes: 35432, cover: null, genre: 'Industrial' },
        { id: 35, title: 'Синтетический рай', artist: 'Электро Волна', artistId: 2, album: 'Синтез будущего', duration: 298, plays: 389765, likes: 28765, cover: null, genre: 'Electronica' },
        { id: 36, title: 'Волновая функция', artist: 'Электро Волна', artistId: 2, album: 'Синтез будущего', duration: 312, plays: 345678, likes: 25678, cover: null, genre: 'Electronica' },
        { id: 37, title: 'Ночная смена', artist: 'Тёмный Ритм', artistId: 3, album: 'Глубина техно', duration: 378, plays: 298765, likes: 23456, cover: null, genre: 'Techno' },
        { id: 38, title: 'Пульс мегаполиса', artist: 'Тёмный Ритм', artistId: 3, album: 'Глубина техно', duration: 401, plays: 267890, likes: 21098, cover: null, genre: 'Techno' },
        { id: 39, title: 'Голливудские огни', artist: 'Неоновые Сны', artistId: 4, album: '80-е возвращаются', duration: 267, plays: 234567, likes: 18765, cover: null, genre: 'Synthwave' },
        { id: 40, title: 'Магнитная лента', artist: 'Неоновые Сны', artistId: 4, album: '80-е возвращаются', duration: 245, plays: 198765, likes: 16543, cover: null, genre: 'Synthwave' }
    ],

    playlists: [
        {
            id: 1,
            title: 'Индустриальная мощь',
            description: 'Лучшие треки индастриала и тяжёлой электроники',
            cover: null,
            owner: 'Кирпич Мастер',
            ownerId: 1,
            trackCount: 25,
            duration: 6789,
            isPublic: true,
            collaborative: false,
            tracks: [1, 2, 17, 18, 33, 34, 5, 6, 21, 22]
        },
        {
            id: 2,
            title: 'Синтвейв ностальгия',
            description: 'Возвращение в 80-е через синтезаторы',
            cover: null,
            owner: 'Неоновые Сны',
            ownerId: 4,
            trackCount: 18,
            duration: 4567,
            isPublic: true,
            collaborative: false,
            tracks: [7, 8, 23, 24, 39, 40, 13, 14]
        },
        {
            id: 3,
            title: 'Глубокий техно',
            description: 'Для настоящих ценителей тёмного техно',
            cover: null,
            owner: 'Тёмный Ритм',
            ownerId: 3,
            trackCount: 22,
            duration: 8234,
            isPublic: true,
            collaborative: false,
            tracks: [5, 6, 21, 22, 37, 38]
        },
        {
            id: 4,
            title: 'Космическое путешествие',
            description: 'Эмбиент для медитаций и релаксации',
            cover: null,
            owner: 'Космический Эмбиент',
            ownerId: 8,
            trackCount: 15,
            duration: 7123,
            isPublic: true,
            collaborative: false,
            tracks: [15, 16, 31, 32, 11, 12, 27, 28]
        },
        {
            id: 5,
            title: 'Басс-линия',
            description: 'Максимум низких частот',
            cover: null,
            owner: 'Тяжёлый Бас',
            ownerId: 5,
            trackCount: 20,
            duration: 5890,
            isPublic: true,
            collaborative: false,
            tracks: [9, 10, 25, 26]
        },
        {
            id: 6,
            title: 'Киберпанк 2077',
            description: 'Саундтрек к нашему будущему',
            cover: null,
            owner: 'Киберпанк',
            ownerId: 7,
            trackCount: 19,
            duration: 6234,
            isPublic: true,
            collaborative: false,
            tracks: [13, 14, 29, 30]
        },
        {
            id: 7,
            title: 'Утренний заряд',
            description: 'Энергичная электроника для начала дня',
            cover: null,
            owner: 'Электро Волна',
            ownerId: 2,
            trackCount: 16,
            duration: 4567,
            isPublic: true,
            collaborative: false,
            tracks: [3, 4, 19, 20, 35, 36]
        },
        {
            id: 8,
            title: 'Ночной драйв',
            description: 'Музыка для ночных поездок по городу',
            cover: null,
            owner: 'Алексей',
            ownerId: 9,
            trackCount: 24,
            duration: 6789,
            isPublic: true,
            collaborative: false,
            tracks: [7, 8, 23, 24, 13, 14, 29, 30, 39, 40]
        },
        {
            id: 9,
            title: 'Рабочий фокус',
            description: 'Концентрация через звук',
            cover: null,
            owner: 'Мария',
            ownerId: 10,
            trackCount: 21,
            duration: 7890,
            isPublic: true,
            collaborative: false,
            tracks: [11, 12, 27, 28, 15, 16, 31, 32]
        },
        {
            id: 10,
            title: 'Танцпол',
            description: 'Всё для зажигательной вечеринки',
            cover: null,
            owner: 'Катя',
            ownerId: 12,
            trackCount: 28,
            duration: 8123,
            isPublic: true,
            collaborative: false,
            tracks: [3, 4, 19, 20, 9, 10, 25, 26]
        }
    ],

    ircMessages: []
};

const ircAuthors = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15];
const ircChannels = ['#general', '#music-production', '#showcase', '#random', '#support'];
const ircMessageTemplates = [
    'Привет всем! 👋',
    'Кто-нибудь слушал новый трек {artist}?',
    'Только что закончил работу над новым битом',
    '@{user} отличная работа!',
    'Какой ваш любимый синтезатор?',
    'Где вы обычно находите вдохновение?',
    'Сегодня отличный день для создания музыки',
    'Кто-то хочет поколлаборировать?',
    'Послушайте мой новый трек в плейлисте',
    '@{user} что думаешь об этом звуке?',
    'Индастриал - это не просто жанр, это образ жизни',
    'Кто-нибудь идёт на фестиваль в этом году?',
    'Только что купил новый модуляр',
    'Работаю над эмбиент-альбомом',
    'Басс должен быть физически ощутимым',
    '@{user} спасибо за поддержку!',
    'Ищу вокалиста для проекта',
    'Какие плагины вы используете?',
    'Аналоговый звук всегда лучше цифрового',
    'Или может быть наоборот? 😄',
    'Кто-то использует аппаратные синтезаторы?',
    'Техно - это состояние души',
    'Синтвейв возвращает нас в прошлое',
    '@{user} давай замутим трек вместе',
    'Только что выпустил новый релиз!',
    'Спасибо всем за поддержку! 🙏',
    'Работаю всю ночь над новым материалом',
    'Кофе и синтезаторы - всё что нужно',
    'Какой жанр вы сейчас слушаете?',
    '@{user} ты где пропал?'
];

for (let i = 0; i < 200; i++) {
    const userId = Utils.randomItem(ircAuthors);
    const user = MockData.users.find(u => u.id === userId);
    const channel = Utils.randomItem(ircChannels);
    let text = Utils.randomItem(ircMessageTemplates);
    
    if (text.includes('{artist}')) {
        const artist = Utils.randomItem(MockData.users.filter(u => u.type === 'artist'));
        text = text.replace('{artist}', artist.displayName);
    }
    
    if (text.includes('{user}')) {
        const randomUser = Utils.randomItem(MockData.users);
        text = text.replace('{user}', randomUser.username);
    }
    
    const timestamp = new Date(Date.now() - Utils.randomInt(0, 7 * 24 * 60 * 60 * 1000));
    
    MockData.ircMessages.push({
        id: i + 1,
        userId,
        username: user.username,
        displayName: user.displayName,
        avatar: user.avatar,
        channel,
        text,
        timestamp: timestamp.toISOString()
    });
}

MockData.ircMessages.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));

window.MockData = MockData;
