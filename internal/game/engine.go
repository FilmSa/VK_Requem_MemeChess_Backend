package game

type MoveResult struct {
	FEN          string       `json:"fen"`
	Move         string       `json:"move"`
	IsCapture    bool         `json:"is_capture"`
	IsCheck      bool         `json:"is_check"`
	IsCheckmate  bool         `json:"is_checkmate"`
	MemeID       string       `json:"meme_id,omitempty"`
	MemeCategory string       `json:"meme_category,omitempty"`
	Effects      []MoveEffect `json:"effects,omitempty"`
}

type Engine interface {
	CurrentFEN() string
	ApplyMove(move string) (MoveResult, error)
}
