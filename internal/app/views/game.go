package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

const blinkLogo = ` ____  _     ___ _   _ _  __
| __ )| |   |_ _| \ | | |/ /
|  _ \| |    | ||  \| | ' /
| |_) | |___ | || |\  | . \
|____/|_____|___|_| \_|_|\_\`

// GameState is the read-only snapshot the view needs.
type GameState struct {
	PlayerY   int
	Items     []GameItem
	Score     int
	HighScore int
	Orbs      int
	GameOver  bool
	Started   bool
}

// GameItem is an object for rendering.
type GameItem struct {
	X    int
	Y    int // height above ground
	Kind int // 0=orb, 1=snake, 2=demon
}

const (
	gHeight   = 12 // total grid rows
	gGround   = gHeight - 1 // row 11 = terrain line
	gPlayerX  = 10
	gMaxFloat = 7
)

// Game renders the Blink Run mini-game.
func Game(gs GameState, t *theme.Theme, width, height int) string {
	gw := clamp(width-8, 40, 80)

	primary := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	accent := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextDim)
	sparkS := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	coreS := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	orbS := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	snakeS := lipgloss.NewStyle().Foreground(t.Success)
	demonS := lipgloss.NewStyle().Foreground(t.Error).Bold(true)
	textS := lipgloss.NewStyle().Foreground(t.Text)

	var b strings.Builder

	// Header
	title := primary.Render("BLINK RUN")
	scoreStr := accent.Render(fmt.Sprintf("Score: %d", gs.Score))
	orbStr := orbS.Render(fmt.Sprintf("  Orbs: %d", gs.Orbs))
	hiStr := ""
	if gs.HighScore > 0 {
		hiStr = muted.Render(fmt.Sprintf("  HI: %d", gs.HighScore))
	}
	right := scoreStr + orbStr + hiStr
	gap := gw - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	b.WriteString(title + strings.Repeat(" ", gap) + right + "\n\n")

	if gs.GameOver {
		renderLogo(&b, blinkLogo, demonS, gw)
		b.WriteString("\n")
		centerLine(&b, demonS.Render("GAME OVER"), gw)
		centerLine(&b, accent.Render(fmt.Sprintf("Score: %d  Orbs: %d", gs.Score, gs.Orbs)), gw)
		b.WriteString("\n")
		centerLine(&b, muted.Render("down")+textS.Render(": restart  ")+muted.Render("esc")+textS.Render(": back"), gw)
	} else if !gs.Started {
		renderLogo(&b, blinkLogo, primary, gw)
		b.WriteString("\n")
		centerLine(&b, accent.Render("Dive to collect orbs. Avoid the darkness!"), gw)
		b.WriteString("\n")
		centerLine(&b, muted.Render("down")+textS.Render(": dive  ")+muted.Render("up")+textS.Render(": float  ")+muted.Render("esc")+textS.Render(": back"), gw)
	} else {
		grid := makeGrid(gHeight, gw)

		// Terrain
		for x := 0; x < gw; x++ {
			grid[gGround][x] = '_'
		}

		// Items at various heights
		for _, it := range gs.Items {
			if it.X < 0 || it.X >= gw {
				continue
			}
			// Convert item Y (height above ground) to grid row
			baseRow := gGround - 1 - it.Y
			switch it.Kind {
			case 0: // orb
				drawOrbAt(grid, it.X, baseRow, gw, gHeight)
			case 1: // snake
				drawSnakeAt(grid, it.X, baseRow, gw, gHeight)
			case 2: // demon
				drawDemonAt(grid, it.X, baseRow, gw, gHeight)
			}
		}

		// Player: the Blink — floating at playerY above ground
		//   *
		//  <@>
		//   *
		feetRow := gGround - 1 - gs.PlayerY
		drawBlink(grid, feetRow, gPlayerX, gw, gHeight)

		// Render with colors
		for _, row := range grid {
			var line strings.Builder
			for _, ch := range row {
				switch ch {
				case '@':
					line.WriteString(coreS.Render(string(ch)))
				case '<', '>':
					line.WriteString(coreS.Render(string(ch)))
				case '*':
					line.WriteString(sparkS.Render(string(ch)))
				case 'o', '(':
					line.WriteString(orbS.Render(string(ch)))
				case ')':
					line.WriteString(orbS.Render(string(ch)))
				case '~', 's':
					line.WriteString(snakeS.Render(string(ch)))
				case 'V', '\\', '/', '^':
					line.WriteString(demonS.Render(string(ch)))
				case '_':
					line.WriteString(muted.Render(string(ch)))
				default:
					line.WriteRune(ch)
				}
			}
			b.WriteString(line.String() + "\n")
		}

		controls := muted.Render("space") + textS.Render(": dive  ") +
			muted.Render("esc") + textS.Render(": back")
		b.WriteString(controls)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(clamp(width-2, 44, gw+8)).
		Render(b.String())
}

// drawBlink places the spark/eye player.
//
//	  *
//	 <@>
//	  *
func drawBlink(grid [][]rune, feetRow, col, w, h int) {
	set := func(r, c int, ch rune) {
		if r >= 0 && r < h && c >= 0 && c < w {
			grid[r][c] = ch
		}
	}
	set(feetRow-2, col, '*')
	set(feetRow-1, col-1, '<')
	set(feetRow-1, col, '@')
	set(feetRow-1, col+1, '>')
	set(feetRow, col, '*')
}

// drawOrbAt places a light orb at the given row.
//
//	(o)
func drawOrbAt(grid [][]rune, x, row, w, h int) {
	set := func(r, c int, ch rune) {
		if r >= 0 && r < h && c >= 0 && c < w {
			grid[r][c] = ch
		}
	}
	set(row, x-1, '(')
	set(row, x, 'o')
	set(row, x+1, ')')
}

// drawSnakeAt places a snake at the given row (1 row tall).
//
//	~s~
func drawSnakeAt(grid [][]rune, x, row, w, h int) {
	set := func(r, c int, ch rune) {
		if r >= 0 && r < h && c >= 0 && c < w {
			grid[r][c] = ch
		}
	}
	set(row, x-1, '~')
	set(row, x, 's')
	set(row, x+1, '~')
}

// drawDemonAt places a demon at the given row (2 rows tall, extends upward).
//
//	 ^
//	\V/
func drawDemonAt(grid [][]rune, x, row, w, h int) {
	set := func(r, c int, ch rune) {
		if r >= 0 && r < h && c >= 0 && c < w {
			grid[r][c] = ch
		}
	}
	set(row-1, x, '^')
	set(row, x-1, '\\')
	set(row, x, 'V')
	set(row, x+1, '/')
}

func makeGrid(rows, cols int) [][]rune {
	g := make([][]rune, rows)
	for i := range g {
		g[i] = make([]rune, cols)
		for j := range g[i] {
			g[i][j] = ' '
		}
	}
	return g
}

func renderLogo(b *strings.Builder, logo string, style lipgloss.Style, w int) {
	for _, line := range strings.Split(logo, "\n") {
		rendered := style.Render(line)
		pad := (w - lipgloss.Width(rendered)) / 2
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad) + rendered + "\n")
	}
}

func centerLine(b *strings.Builder, s string, w int) {
	pad := (w - lipgloss.Width(s)) / 2
	if pad < 0 {
		pad = 0
	}
	b.WriteString(strings.Repeat(" ", pad) + s + "\n")
}
