package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"meme_chess/internal/analyzer"
	"meme_chess/internal/auth"
	"meme_chess/internal/bootstrap"
	"meme_chess/internal/config"
	"meme_chess/internal/db"
	"meme_chess/internal/game"
	"meme_chess/internal/inventory"
	"meme_chess/internal/shop"
	"meme_chess/internal/user"
	"meme_chess/internal/ws"
)

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		} else {
			w.Header().Add("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func registerAuthRoutes(h *auth.Handlers) {
	routes := []struct {
		path    string
		handler http.HandlerFunc
	}{
		{path: "/auth/register", handler: h.Register},
		{path: "/auth/login", handler: h.Login},
		{path: "/auth/me", handler: h.Me},
		{path: "/auth/avatar", handler: h.UploadAvatar},
		{path: "/auth/currency", handler: h.Currency},
		{path: "/auth/logout", handler: h.Logout},
		{path: "/api/v1/auth/register", handler: h.Register},
		{path: "/api/v1/auth/login", handler: h.Login},
		{path: "/api/v1/auth/me", handler: h.Me},
		{path: "/api/v1/auth/avatar", handler: h.UploadAvatar},
		{path: "/api/v1/auth/currency", handler: h.Currency},
		{path: "/api/v1/auth/logout", handler: h.Logout},
	}

	for _, route := range routes {
		http.HandleFunc(route.path, route.handler)
	}
}

func main() {
	cfg := config.Load()

	pg, err := db.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	defer pg.Pool.Close()

	if err := db.RunMigrations(pg.Pool, "migrations"); err != nil {
		log.Fatalf("failed to apply migrations: %v", err)
	}

	if err := bootstrap.SyncStoreCatalog(pg.Pool); err != nil {
		log.Fatalf("failed to sync store catalog: %v", err)
	}

	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	userRepo := user.NewRepository(pg.Pool)
	authService := auth.NewService(userRepo, jwtManager)
	authHandlers := &auth.Handlers{
		Service:    authService,
		UploadsDir: cfg.UploadsDir,
	}

	hub := ws.NewHub()
	gameRepo := game.NewRepository(pg.Pool)
	gameService := game.NewService(gameRepo)
	gameService.SetUserRepository(userRepo)
	invRepo := inventory.NewRepository(pg.Pool)
	invService := inventory.NewService(invRepo)
	invHTTP := &inventory.HTTP{Svc: invService, Auth: authService}
	inventory.RegisterRoutes(http.DefaultServeMux, invHTTP)

	shopRepo := shop.NewRepository(pg.Pool)
	shopService := shop.NewService(shopRepo, userRepo)
	shopHTTP := &shop.HTTP{Svc: shopService, Auth: authService}
	shop.RegisterRoutes(http.DefaultServeMux, shopHTTP)

	wsHandler := ws.NewHandler(hub, gameService, invService, jwtManager)
	gameHTTP := &game.HTTP{
		Svc:               gameService,
		JWT:               jwtManager,
		AuthService:       authService,
		UserRepo:          userRepo,
		InventoryService:  invService,
		JoinBase:          cfg.FrontendJoinBase,
		BroadcastState:    hub.BroadcastGameState,
		BroadcastFinished: hub.BroadcastGameFinished,
	}
	analyzerHTTP, gameAnalyzer, err := analyzer.NewHTTPHandler(pg.Pool)
	if err != nil {
		log.Fatalf("failed to initialize analyzer: %v", err)
	}
	gameService.SetMoveAnalyzer(gameAnalyzer)

	go hub.Run()

	registerAuthRoutes(authHandlers)
	http.Handle("/api/v1/", analyzerHTTP)
	http.HandleFunc("/api/v1/games/invite", gameHTTP.PostInvite)
	http.HandleFunc("/api/v1/games/robot", gameHTTP.PostRobotGame)
	http.HandleFunc("/api/v1/games/match-search", gameHTTP.PostMatchSearch)
	http.HandleFunc("/api/v1/games/match-search/preview", gameHTTP.PostMatchSearchPreview)
	http.HandleFunc("/api/v1/games/match-search/leave", gameHTTP.PostMatchSearchLeave)
	http.HandleFunc("/api/v1/games/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/games/")
		rest = strings.Trim(rest, "/")
		parts := strings.Split(rest, "/")
		if len(parts) == 1 && parts[0] == "history" {
			gameHTTP.GetMyGameHistory(w, r)
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "participants" {
			gameHTTP.GetParticipants(w, r, parts[0])
			return
		}
		if len(parts) == 3 && parts[0] != "" && parts[1] == "analysis" && parts[2] == "move" {
			gameHTTP.PostAnalyzeMove(w, r, parts[0])
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "resign" {
			gameHTTP.PostResign(w, r, parts[0])
			return
		}
		if len(parts) == 2 && parts[0] != "" && parts[1] == "timeout" {
			gameHTTP.PostTimeout(w, r, parts[0])
			return
		}
		if len(parts) == 3 && parts[0] != "" && parts[1] == "draw" && parts[2] == "offer" {
			gameHTTP.PostDrawOffer(w, r, parts[0])
			return
		}
		if len(parts) == 3 && parts[0] != "" && parts[1] == "draw" && parts[2] == "accept" {
			gameHTTP.PostDrawAccept(w, r, parts[0])
			return
		}
		if len(parts) == 3 && parts[0] != "" && parts[1] == "draw" && parts[2] == "decline" {
			gameHTTP.PostDrawDecline(w, r, parts[0])
			return
		}
		http.NotFound(w, r)
	})
	http.HandleFunc("/api/v1/invites/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/invites/")
		rest = strings.Trim(rest, "/")
		parts := strings.Split(rest, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] != "join" {
			http.NotFound(w, r)
			return
		}

		gameHTTP.PostInviteJoin(w, r, parts[0])
	})

	http.Handle("/emoji/", http.StripPrefix("/emoji/", http.FileServer(http.Dir("emoji"))))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir))))
	http.HandleFunc("/games/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/games/")
		rest = strings.TrimSuffix(rest, "/")
		if strings.Contains(rest, "/") {
			http.NotFound(w, r)
			return
		}
		if rest == "invite" {
			gameHTTP.PostInvite(w, r)
			return
		}
		if rest == "history" {
			gameHTTP.GetMyGameHistory(w, r)
			return
		}
		if rest == "" {
			http.NotFound(w, r)
			return
		}
		gameHTTP.GetGame(w, r, rest)
	})

	http.HandleFunc("/ws", wsHandler.ServeWS)

	http.HandleFunc("/debug/token", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}

		token, err := jwtManager.Generate(userID)
		if err != nil {
			http.Error(w, "failed to generate token", http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]string{
			"token": token,
		})
	})

	log.Printf("server started on :%s", cfg.HTTPPort)
	//log.Fatal(http.ListenAndServe(":"+cfg.HTTPPort, nil))
	log.Fatal(http.ListenAndServe(":"+cfg.HTTPPort, withCORS(http.DefaultServeMux)))
}
