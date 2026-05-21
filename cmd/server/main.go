package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/bogdan/kirpichmusic/internal/admin"
	"github.com/bogdan/kirpichmusic/internal/artists"
	"github.com/bogdan/kirpichmusic/internal/auth"
	"github.com/bogdan/kirpichmusic/internal/comments"
	"github.com/bogdan/kirpichmusic/internal/config"
	dbpkg "github.com/bogdan/kirpichmusic/internal/db"
	"github.com/bogdan/kirpichmusic/internal/follows"
	"github.com/bogdan/kirpichmusic/internal/httpx"
	"github.com/bogdan/kirpichmusic/internal/playlists"
	"github.com/bogdan/kirpichmusic/internal/storage"
	"github.com/bogdan/kirpichmusic/internal/tracks"
	"github.com/bogdan/kirpichmusic/internal/users"
)

func main() {
	// Если рядом есть .env — подгружаем его как fallback (реальные env
	// имеют приоритет). В проде .env обычно нет — переменные приходят
	// от systemd/k8s/docker, и эта строка просто молча no-op.
	config.LoadDotEnv(".env")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := dbpkg.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	sessions := auth.NewSessionStore(pool, cfg.SessionCookie, cfg.CookieDomain, cfg.CookieSecure, cfg.SessionTTL)
	authH := auth.NewHandler(pool, sessions)

	store, err := storage.New(cfg.MediaRoot, cfg.MaxUploadBytes)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	tracksRepo := tracks.NewRepository(pool)
	tracksH := tracks.NewHandler(tracksRepo, store)

	commentsH := comments.NewHandler(comments.NewRepository(pool))
	playlistsH := playlists.NewHandler(playlists.NewRepository(pool))
	usersH := users.NewHandler(pool, store)

	artistsRepo := artists.NewRepository(pool)
	artistsH := artists.NewHandler(artistsRepo, pool, store)
	tracksH.SetArtistsRepo(artistsRepo)

	adminH := admin.NewHandler(pool, cfg.MediaRoot, "")

	followsRepo := follows.NewRepository(pool)
	followsH := follows.NewHandler(followsRepo)
	// Лёгкий refresh топ-юзеров на старте (если БД успела набрать прослушиваний).
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := followsRepo.Refresh(ctx); err != nil {
			log.Printf("initial refresh top_uploaders: %v", err)
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(httpx.SecurityHeaders(cfg.CookieSecure))
	// Без глобального Timeout — он бы срезал длинный аудио-стрим.
	// Таймаут навешиваем точечно на короткие JSON-эндпоинты.

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api/auth", func(r chi.Router) {
		r.Use(middleware.Timeout(15 * time.Second))
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)
		r.Post("/logout", authH.Logout)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(sessions))
			r.Get("/me", authH.Me)
			r.Patch("/me", authH.UpdateMe)
			r.Delete("/me", authH.DeleteMe)
		})
		// Аватар/шапка — multipart upload, повышенный таймаут.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(sessions))
			r.Use(middleware.Timeout(2 * time.Minute))
			r.Post("/me/avatar", usersH.UploadAvatar)
			r.Delete("/me/avatar", usersH.DeleteAvatar)
			r.Post("/me/banner", usersH.UploadBanner)
			r.Delete("/me/banner", usersH.DeleteBanner)
		})
	})

	// Статический фронтенд из cfg.WebRoot. http.FileServer сам выдаёт index.html
	// по запросу к "/" и блокирует выход за пределы директории (path-cleaning).
	fs := http.FileServer(http.Dir(cfg.WebRoot))
	r.Handle("/", fs)
	r.Handle("/index.html", fs)
	// Любые статические assets, если появятся (CSS/JS/images внутри ./web).
	r.Handle("/static/*", fs)

	// SPA-маршруты (deep-links): /user/<username>, /admin, /profile, /library.
	// Отдаём index.html — JS прочитает location.pathname и откроет нужную страницу.
	indexFn := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, cfg.WebRoot+"/index.html")
	}
	r.Get("/user/{username}", indexFn)
	r.Get("/user/{id}/followers", indexFn)
	r.Get("/user/{id}/following", indexFn)
	r.Get("/artist/{slug}", indexFn)
	r.Get("/admin", indexFn)
	r.Get("/profile", indexFn)
	r.Get("/library", indexFn)
	r.Get("/home", indexFn)

	r.Route("/api/tracks", func(r chi.Router) {
		// Публичные JSON-эндпоинты с OptionalAuth (для liked_by_me).
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Use(auth.OptionalAuth(sessions, pool))
			r.Get("/", tracksH.List)
			r.Get("/{id}", tracksH.GetOne)
			r.Get("/{id}/comments", commentsH.List)
		})
		// Стриминг audio/cover — БЕЗ Timeout.
		// http.ServeContent использует Range и может отдавать файл часами.
		r.Get("/{id}/audio", tracksH.ServeAudio)
		r.Get("/{id}/cover", tracksH.ServeCover)
		// Upload + лайки + комменты — auth.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(sessions))
			r.Use(middleware.Timeout(5 * time.Minute))
			r.Post("/", tracksH.Upload)
		})
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(sessions))
			r.Use(middleware.Timeout(15 * time.Second))
			r.Post("/{id}/like", tracksH.Like)
			r.Delete("/{id}/like", tracksH.Unlike)
			r.Post("/{id}/comments", commentsH.Add)
			r.Delete("/{id}", tracksH.Delete)
		})
		// PATCH = multipart с возможной новой обложкой → отдельный таймаут.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(sessions))
			r.Use(middleware.Timeout(2 * time.Minute))
			r.Patch("/{id}", tracksH.Edit)
		})
	})

	// Плейлисты.
	r.Route("/api/playlists", func(r chi.Router) {
		r.Use(middleware.Timeout(15 * time.Second))
		// Чтение — публичное (с OptionalAuth для is_own / приватных).
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuth(sessions, pool))
			r.Get("/", playlistsH.List)
			r.Get("/{id}", playlistsH.Get)
		})
		// Мутации — только владелец.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(sessions))
			r.Post("/", playlistsH.Create)
			r.Patch("/{id}", playlistsH.Update)
			r.Delete("/{id}", playlistsH.Delete)
			r.Post("/{id}/tracks", playlistsH.AddTrack)
			r.Delete("/{id}/tracks/{track_id}", playlistsH.RemoveTrack)
		})
	})

	r.Route("/api/users", func(r chi.Router) {
		// Стриминг картинок — без таймаута (он бы срезал большие файлы).
		r.Get("/{id}/avatar", usersH.ServeAvatar)
		r.Get("/{id}/banner", usersH.ServeBanner)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Use(auth.OptionalAuth(sessions, pool))
			r.Get("/top", followsH.Top)
			r.Get("/by-username/{username}", usersH.GetByUsername)
			r.Get("/{id}", usersH.GetByID)
			r.Get("/{id}/followers", usersH.GetFollowers)
			r.Get("/{id}/following", usersH.GetFollowing)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Use(auth.RequireAuth(sessions))
			r.Post("/{id}/follow", followsH.Follow)
			r.Delete("/{id}/follow", followsH.Unfollow)
		})
	})

	// Артисты — отдельная сущность от пользователей.
	r.Route("/api/artists", func(r chi.Router) {
		// Стриминг картинок — без таймаута.
		r.Get("/{id}/avatar", artistsH.ServeAvatar)
		r.Get("/{id}/banner", artistsH.ServeBanner)
		r.Get("/proposals/{pid}/image", artistsH.ServeProposalImage)

		// Публичные JSON-эндпоинты.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Use(auth.OptionalAuth(sessions, pool))
			r.Get("/", artistsH.List)
			r.Get("/suggest", artistsH.Suggest)
			r.Get("/by-slug/{slug}", artistsH.GetBySlug)
			r.Get("/{id}", artistsH.GetByID)
		})

		// PATCH — только claimed-юзер ИЛИ админ (проверка внутри хендлера).
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(15 * time.Second))
			r.Use(auth.RequireAuth(sessions))
			r.Patch("/{id}", artistsH.Patch)
		})
		// Upload avatar/banner — только claimed-юзер ИЛИ админ.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(2 * time.Minute))
			r.Use(auth.RequireAuth(sessions))
			r.Post("/{id}/avatar", artistsH.UploadAvatar)
			r.Delete("/{id}/avatar", artistsH.DeleteAvatar)
			r.Post("/{id}/banner", artistsH.UploadBanner)
			r.Delete("/{id}/banner", artistsH.DeleteBanner)
		})
		// Народные предложения аватарок — любой залогиненный.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(2 * time.Minute))
			r.Use(auth.RequireAuth(sessions))
			r.Post("/{id}/proposals", artistsH.SubmitProposal)
		})
	})

	// Админ-эндпоинты. RequireAdmin сама проверяет cookie + is_admin=TRUE.
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.Timeout(10 * time.Second))
		r.Use(auth.RequireAdmin(sessions, pool))
		r.Get("/uptime", adminH.Uptime)

		// Очередь предложений аватарок.
		r.Get("/artists/proposals", artistsH.AdminListProposals)
		r.Post("/artists/proposals/{pid}/approve", artistsH.AdminApproveProposal)
		r.Post("/artists/proposals/{pid}/reject", artistsH.AdminRejectProposal)

		// Привязка артиста к user'у.
		r.Post("/artists/{id}/claim", artistsH.AdminClaim)

		// Управление пользователями.
		r.Get("/users", adminH.ListUsers)
		r.Patch("/users/{id}", adminH.PatchUser)

		// Очередь модерации треков (для верифицированных артистов).
		r.Get("/tracks/pending", adminH.ListPendingTracks)
		r.Post("/tracks/{id}/approve", adminH.ApproveTrack)
		r.Post("/tracks/{id}/reject", adminH.RejectTrack)
		// Удаление любого трека админом.
		r.Delete("/tracks/{id}", adminH.DeleteTrack)
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		// ReadTimeout/WriteTimeout не задаём — иначе срежем upload и стриминг
		// крупных файлов. Безопасность достигается на уровне handlers
		// (MaxBytesReader, RequireAuth, точечные middleware.Timeout).
		IdleTimeout: 2 * time.Minute,
	}

	go func() {
		log.Printf("KirpichMusic API listening on %s (env=%s)", cfg.HTTPAddr, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
