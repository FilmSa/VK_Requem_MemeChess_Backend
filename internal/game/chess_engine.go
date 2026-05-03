package game

type engineRuntime interface {
	CurrentFEN() string
	ApplyMove(move string) (MoveResult, error)
}

type ChessEngine struct {
	runtime engineRuntime
}

func NewChessEngine() *ChessEngine {
	engine, err := NewChessEngineForMode(GameModeClassic)
	if err != nil {
		panic(err)
	}
	return engine
}

func NewChessEngineForMode(mode string) (*ChessEngine, error) {
	return newChessEngineForMode(mode, cryptoRandomizer{})
}

func newChessEngineForMode(mode string, rng randomizer) (*ChessEngine, error) {
	runtime, err := newEngineRuntime(mode, rng)
	if err != nil {
		return nil, err
	}
	return &ChessEngine{runtime: runtime}, nil
}

func (e *ChessEngine) CurrentFEN() string {
	return e.runtime.CurrentFEN()
}

func (e *ChessEngine) ApplyMove(uciMove string) (MoveResult, error) {
	return e.runtime.ApplyMove(uciMove)
}

func (e *ChessEngine) LegalMoves() []string {
	type legalMoveProvider interface {
		LegalMoves() []string
	}

	provider, ok := e.runtime.(legalMoveProvider)
	if !ok {
		return nil
	}

	return provider.LegalMoves()
}
