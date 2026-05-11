package game

import (
	"fmt"
	"strings"
)

type MockEngine struct {
	fen       string
	moveCount int
}

func NewMockEngine() *MockEngine {
	return &MockEngine{
		fen: "startpos",
	}
}

func (e *MockEngine) CurrentFEN() string {
	return e.fen
}

func (e *MockEngine) ApplyMove(move string) (MoveResult, error) {
	move = strings.TrimSpace(move)
	if move == "" {
		return MoveResult{}, ErrInvalidMove
	}

	e.moveCount++

	isCapture := strings.Contains(move, "x")
	isCheck := strings.Contains(move, "+")
	isCheckmate := strings.Contains(move, "#")

	e.fen = fmt.Sprintf("mock-fen-after-%d", e.moveCount)

	return MoveResult{
		FEN:         e.fen,
		Move:        move,
		IsCapture:   isCapture,
		IsCheck:     isCheck,
		IsCheckmate: isCheckmate,
		Effects:     nil,
	}, nil
}
