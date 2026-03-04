package app

import (
	"math/rand"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Game tuning constants.
const (
	gameTickRate  = 60 * time.Millisecond
	gamePlayerX  = 10
	gameMaxH     = 7 // max float height
	gameDiveSpd  = 2
	gameFloatSpd = 1
	gameOrbScore = 10
)

type gameTickMsg struct{}

// itemKind distinguishes objects.
type itemKind int

const (
	itemOrb   itemKind = iota // collectible light orb
	itemSnake                 // enemy, 1 row tall
	itemDemon                 // enemy, 2 rows tall
)

type gameItem struct {
	x    int
	y    int // height above ground (0 = ground level, higher = in the air)
	kind itemKind
}

func (it gameItem) collisionHeight() int {
	switch it.kind {
	case itemDemon:
		return 2
	default:
		return 1
	}
}

// gameState is fully self-contained game logic.
type gameState struct {
	playerY    int
	divingDown bool
	rising     bool
	items      []gameItem
	orbs       int
	score      int
	highScore  int
	gameOver   bool
	started    bool
	ticks      int
	width      int
	rng        *rand.Rand
}

func newGameState() gameState {
	return gameState{
		playerY: gameMaxH,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// HandleKey processes input. Returns a tea.Cmd if the game loop should start.
func (g *gameState) HandleKey(key string) tea.Cmd {
	switch key {
	case "down":
		if g.gameOver {
			g.restart()
			return gameTickCmd()
		}
		if !g.started {
			g.started = true
			g.dive()
			return gameTickCmd()
		}
		g.dive()
	case "up":
		if g.gameOver {
			g.restart()
			return gameTickCmd()
		}
		if !g.started {
			g.started = true
			return gameTickCmd()
		}
		if g.divingDown {
			g.divingDown = false
			g.rising = true
		}
	}
	return nil
}

// Tick advances one frame. Returns true if the game is still running.
func (g *gameState) Tick(gameWidth int) bool {
	if g.gameOver || !g.started {
		return false
	}
	g.width = gameWidth
	g.ticks++
	g.score = g.ticks/3 + g.orbs*gameOrbScore

	// Player movement
	if g.divingDown {
		g.playerY -= gameDiveSpd
		if g.playerY <= 0 {
			g.playerY = 0
			g.divingDown = false
			g.rising = true
		}
	} else if g.rising {
		g.playerY += gameFloatSpd
		if g.playerY >= gameMaxH {
			g.playerY = gameMaxH
			g.rising = false
		}
	}

	// Move items left
	speed := 1 + g.ticks/200
	alive := g.items[:0]
	for _, it := range g.items {
		it.x -= speed
		if it.x > -3 {
			alive = append(alive, it)
		}
	}
	g.items = alive

	// Spawn items
	spawnInterval := max(6, 14-g.ticks/80)
	if g.ticks%spawnInterval == 0 {
		g.spawnItem(gameWidth)
	}

	// Collisions
	for i, it := range g.items {
		if it.x < gamePlayerX-1 || it.x > gamePlayerX+1 {
			continue
		}
		// Player occupies y range: [playerY - collisionMargin, playerY + collisionMargin]
		// Item occupies y range: [it.y, it.y + it.collisionHeight())
		// Overlap if player is within item's vertical range
		if g.playerY >= it.y && g.playerY < it.y+it.collisionHeight() {
			switch it.kind {
			case itemOrb:
				g.orbs++
				g.items[i].x = -99
			case itemSnake, itemDemon:
				g.gameOver = true
				if g.score > g.highScore {
					g.highScore = g.score
				}
				return false
			}
		}
	}

	return true
}

func (g *gameState) dive() {
	if !g.divingDown {
		g.divingDown = true
		g.rising = false
	}
}

func (g *gameState) spawnItem(w int) {
	r := g.rng.Intn(100)
	var kind itemKind
	switch {
	case r < 45:
		kind = itemOrb
	case r < 75:
		kind = itemSnake
	default:
		kind = itemDemon
	}

	// Pick a height: ground (0), low air (2-3), or high air (5-6)
	y := 0
	switch g.rng.Intn(3) {
	case 0:
		y = 0 // ground level
	case 1:
		y = 2 + g.rng.Intn(2) // low air (2-3)
	case 2:
		y = 5 + g.rng.Intn(2) // high air (5-6)
	}

	g.items = append(g.items, gameItem{x: w - 1, y: y, kind: kind})
}

func (g *gameState) restart() {
	hs := g.highScore
	*g = newGameState()
	g.highScore = hs
	g.started = true
}

func gameTickCmd() tea.Cmd {
	return tea.Tick(gameTickRate, func(t time.Time) tea.Msg {
		return gameTickMsg{}
	})
}
