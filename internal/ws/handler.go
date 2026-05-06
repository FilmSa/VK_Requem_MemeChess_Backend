package ws

import (
	"net/http"

	"meme_chess/internal/auth"
	"meme_chess/internal/game"
	"meme_chess/internal/inventory"

	"github.com/gorilla/websocket"
)

type Handler struct {
	hub         *Hub
	gameService *game.Service
	invService  *inventory.Service
	jwtManager  *auth.JWTManager
	upgrader    websocket.Upgrader
}

func NewHandler(hub *Hub, gameService *game.Service, invService *inventory.Service, jwtManager *auth.JWTManager) *Handler {
	return &Handler{
		hub:         hub,
		gameService: gameService,
		invService:  invService,
		jwtManager:  jwtManager,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := h.jwtManager.Parse(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := NewClient(h.hub, h.gameService, h.invService, conn, claims.UserID)
	h.hub.register <- client

	go client.writePump()
	go client.readPump()
}
