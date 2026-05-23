package search

import (
	"meme_chess/internal/analyzer/movegen"
	"meme_chess/internal/analyzer/position"
	"meme_chess/internal/analyzer/rules"
)

const (
	negInf    = -10000000
	posInf    = 10000000
	MateScore = 1000000
)

type Engine struct {
	gen    *movegen.Generator
	rules  rules.RuleSet
	static StaticEvaluator
}

func NewEngine(rs rules.RuleSet) *Engine {
	return &Engine{
		gen:    movegen.NewGenerator(rs),
		rules:  rs,
		static: NewStaticEvaluator(),
	}
}

func (e *Engine) Analyze(gs *position.GameState, depth int) *Node {
	result := e.AnalyzePosition(gs, depth)

	children := make([]*Node, 0, len(result.RootMoves))
	for _, mv := range result.RootMoves {
		children = append(children, &Node{
			Hash:     gs.Hash(),
			Move:     mv.Move,
			Score:    mv.Score,
			Depth:    max(0, depth-1),
			Expanded: len(mv.PV) > 0,
			PV:       append([]position.Move(nil), mv.PV...),
		})
	}

	node := &Node{
		Hash:     result.Hash,
		Move:     result.BestMove,
		Score:    result.Score,
		Depth:    result.Depth,
		Expanded: true,
		PV:       append([]position.Move(nil), result.PV...),
		Children: children,
	}

	return node
}

// AnalyzePosition runs iterative deepening and returns one coherent root
// result: best move, principal variation and per-move scores for the entire
// position.
func (e *Engine) AnalyzePosition(gs *position.GameState, depth int) *Result {
	if depth < 1 {
		depth = 1
	}

	hash := gs.Hash()
	key := gs.Key()
	best := &Result{
		Hash:  hash,
		Depth: depth,
	}
	tt := NewTranspositionTable()

	for currentDepth := 1; currentDepth <= depth; currentDepth++ {
		rootMoves := e.gen.GenerateLegalMoves(gs)
		if len(rootMoves) == 0 {
			score := e.terminalScore(gs, 0)
			best = &Result{
				Hash:      hash,
				Score:     score,
				Depth:     currentDepth,
				RootMoves: nil,
			}
			break
		}

		ttMove := position.NullMove()
		if entry, ok := tt.Get(key); ok {
			ttMove = entry.BestMove
		}
		ordered := e.orderMoves(gs, rootMoves, ttMove, 0)

		alpha := negInf
		beta := posInf
		bestScore := negInf
		bestMove := position.NullMove()
		bestPV := []position.Move(nil)
		moveScores := make([]MoveScore, 0, len(ordered))
		nodes := 0

		for i, mv := range ordered {
			if err := gs.ApplyMove(mv); err != nil {
				continue
			}

			var (
				score int
				pv    []position.Move
			)
			if i == 0 {
				score, pv = e.negamax(gs, tt, currentDepth-1, 1, -beta, -alpha, &nodes)
				score = -score
			} else {
				score, pv = e.negamax(gs, tt, currentDepth-1, 1, -alpha-1, -alpha, &nodes)
				score = -score
				if score > alpha && score < beta {
					score, pv = e.negamax(gs, tt, currentDepth-1, 1, -beta, -alpha, &nodes)
					score = -score
				}
			}

			if err := gs.UndoMove(); err != nil {
				panic(err)
			}

			line := append([]position.Move{mv}, pv...)
			moveScores = append(moveScores, MoveScore{
				Move:  mv,
				Score: score,
				PV:    line,
			})

			if score > bestScore {
				bestScore = score
				bestMove = mv
				bestPV = line
			}
			if score > alpha {
				alpha = score
			}
		}

		best = &Result{
			Hash:      hash,
			Score:     bestScore,
			BestMove:  bestMove,
			Depth:     currentDepth,
			PV:        bestPV,
			RootMoves: moveScores,
			Nodes:     nodes,
		}

		tt.Put(TTEntry{
			Hash:     key,
			Depth:    currentDepth,
			Score:    bestScore,
			Bound:    BoundExact,
			BestMove: bestMove,
		})
	}

	return best
}

func (e *Engine) negamax(gs *position.GameState, tt TranspositionTable, depth int, ply int, alpha, beta int, nodes *int) (int, []position.Move) {
	*nodes = *nodes + 1

	key := gs.Key()
	alphaOrig := alpha
	if entry, ok := tt.Get(key); ok && entry.Depth >= depth {
		switch entry.Bound {
		case BoundExact:
			return entry.Score, nil
		case BoundLower:
			if entry.Score > alpha {
				alpha = entry.Score
			}
		case BoundUpper:
			if entry.Score < beta {
				beta = entry.Score
			}
		}
		if alpha >= beta {
			return entry.Score, nil
		}
	}

	if depth == 0 {
		return e.quiescence(gs, 0, alpha, beta, nodes), nil
	}

	inCheck := e.rules.IsCheck(gs, gs.SideToMove)
	if !inCheck && depth >= 3 {
		nullDepth := depth - 3
		if nullDepth < 0 {
			nullDepth = 0
		}

		gs.ApplyNullMove()
		score, _ := e.negamax(gs, tt, nullDepth, ply+1, -beta, -beta+1, nodes)
		score = -score
		if err := gs.UndoMove(); err != nil {
			panic(err)
		}

		if score >= beta {
			return beta, nil
		}
	}

	moves := e.gen.GenerateLegalMoves(gs)
	if len(moves) == 0 {
		return e.terminalScore(gs, ply), nil
	}

	ttMove := position.NullMove()
	if entry, ok := tt.Get(key); ok {
		ttMove = entry.BestMove
	}
	ordered := e.orderMoves(gs, moves, ttMove, ply)

	bestScore := negInf
	bestMove := position.NullMove()
	var bestPV []position.Move

	for i, mv := range ordered {
		tactical := isTacticalMove(gs, mv)
		if err := gs.ApplyMove(mv); err != nil {
			continue
		}
		givesCheck := e.rules.IsCheck(gs, gs.SideToMove)

		var (
			score   int
			childPV []position.Move
		)
		if i == 0 {
			score, childPV = e.negamax(gs, tt, depth-1, ply+1, -beta, -alpha, nodes)
			score = -score
		} else {
			searchDepth := depth - 1
			reduced := false
			if i >= 3 && depth >= 4 && !inCheck && !tactical && !givesCheck {
				searchDepth--
				reduced = true
			}

			score, childPV = e.negamax(gs, tt, searchDepth, ply+1, -alpha-1, -alpha, nodes)
			score = -score
			if reduced && score > alpha {
				score, childPV = e.negamax(gs, tt, depth-1, ply+1, -alpha-1, -alpha, nodes)
				score = -score
			}
			if score > alpha && score < beta {
				score, childPV = e.negamax(gs, tt, depth-1, ply+1, -beta, -alpha, nodes)
				score = -score
			}
		}

		if err := gs.UndoMove(); err != nil {
			panic(err)
		}

		if score > bestScore {
			bestScore = score
			bestMove = mv
			bestPV = append([]position.Move{mv}, childPV...)
		}
		if score > alpha {
			alpha = score
		}
		if alpha >= beta {
			break
		}
	}

	bound := BoundExact
	switch {
	case bestScore <= alphaOrig:
		bound = BoundUpper
	case bestScore >= beta:
		bound = BoundLower
	}

	tt.Put(TTEntry{
		Hash:     key,
		Depth:    depth,
		Score:    bestScore,
		Bound:    bound,
		BestMove: bestMove,
	})

	return bestScore, bestPV
}

func (e *Engine) terminalScore(gs *position.GameState, ply int) int {
	if e.rules.IsCheck(gs, gs.SideToMove) {
		return -MateScore + ply
	}
	return 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
