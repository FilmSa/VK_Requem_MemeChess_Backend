package ws

import "encoding/json"

type IncomingMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type OutgoingMessage struct {
	Type      string      `json:"type"`
	RequestID string      `json:"request_id,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Error     *ErrorBody  `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type JoinGamePayload struct {
	GameID string `json:"game_id"`
}

type GameMovePayload struct {
	GameID string `json:"game_id"`
	Move   string `json:"move"`
}

type GameResignPayload struct {
	GameID string `json:"game_id"`
}

type GameDrawPayload struct {
	GameID string `json:"game_id"`
}

type GameEmotePayload struct {
	GameID    string `json:"game_id"`
	EmoteSlug string `json:"emote_slug,omitempty"`
	EmoteMP4  string `json:"emote_mp4"`
}

type GameStickerPayload struct {
	GameID      string `json:"game_id"`
	StickerSlug string `json:"sticker_slug,omitempty"`
	StickerID   string `json:"sticker_id,omitempty"`
	Title       string `json:"title,omitempty"`
	AssetURL    string `json:"asset_url"`
	MediaType   string `json:"media_type,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	VideoURL    string `json:"video_url,omitempty"`
	SoundURL    string `json:"sound_url,omitempty"`
}
