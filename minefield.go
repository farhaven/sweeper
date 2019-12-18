package main

import (
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
)

type ViewPortElement rune

const (
	// TODO: Make some use of the zero value?

	// Numbers
	VPEZero ViewPortElement = '0' + iota
	VPEOne
	VPETwo
	VPEThree
	VPEFour
	VPEFive
	VPESix
	VPESeven
	VPEEight

	// Others
	VPENone  = ' '
	VPEFlag  = 'P'
	VPEMaybe = '?'
	VPEMine  = 'X' // Only used for debugging (or some spectator mode?)
)

func (ve ViewPortElement) String() string {
	return fmt.Sprintf("%c", ve)
}

type ViewPort struct {
	Position image.Rectangle
	Data     [][]ViewPortElement
}

func NewViewPort(rect image.Rectangle) ViewPort {
	width, height := rect.Dx(), rect.Dy()

	res := ViewPort{}
	res.Position = rect
	res.Data = make([][]ViewPortElement, height)
	for y := 0; y < height; y++ {
		res.Data[y] = make([]ViewPortElement, width)
	}

	return res
}

// IntToBytes converts x to a little endian byte slice
func IntToBytes(val int64) []byte {
	x := uint64(val + math.MinInt64)
	res := [8]byte{}

	for idx := 0; idx < 8; idx++ {
		res[idx] = byte(x >> (8 * idx))
	}

	return res[:]
}

type Mark int

const (
	MarkNone = iota
	MarkFlag
	MarkQuestion
	MarkMax // Not a real value, only used for cycling through marks
)

type MineField struct {
	mu sync.RWMutex

	// Path from which the minefield is read on restart and to which it is saved on changes
	persistencePath string

	Seed    [16]byte
	Density uint32
	// Map of coordinates to neighboring mine count
	Uncovered map[image.Point]int
	// Map of Triggered mines
	Triggered map[image.Point]bool
	// Map of Marks, i.e. flags and question Marks
	Marks map[image.Point]Mark
}

func NewMineField(threshold uint32, persistencePath string) (*MineField, error) {
	fh, err := os.Open(persistencePath)
	if err != nil {
		log.Printf("can't open minefield %s: %s, creating new field", persistencePath, err)

		m := MineField{
			Density:         5,
			Uncovered:       make(map[image.Point]int),
			Triggered:       make(map[image.Point]bool),
			Marks:           make(map[image.Point]Mark),
			persistencePath: persistencePath,
		}
		_, err = rand.Read(m.Seed[:])
		if err != nil {
			return nil, err
		}
		return &m, nil
	}
	defer fh.Close()

	var m MineField
	dec := gob.NewDecoder(fh)
	err = dec.Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("can't load minefield: %w", err)
	}
	return &m, nil
}

func (m *MineField) Persist() error {
	log.Println("persisting minefield")

	m.mu.RLock()
	defer m.mu.RUnlock()

	fh, err := os.Create(m.persistencePath)
	if err != nil {
		return err
	}
	defer fh.Close()

	encoder := gob.NewEncoder(fh)
	err = encoder.Encode(m)
	if err != nil {
		return err
	}

	log.Println("minefield persisted")
	return nil
}

// IsMineOnLocation returns true if there is a mine in the location indicated by x and y.
func (m *MineField) IsMineOnLocation(x, y int) bool {
	// Determine wether x, y contains a mine by hashing it with the seed and checking whether it's less than a threshold
	h := fnv.New32()
	h.Write(m.Seed[:])
	h.Write(IntToBytes(int64(x)))
	h.Write(IntToBytes(int64(y)))
	return (h.Sum32() % m.Density) == 0
}

// ExtractPlayerView returns a 2 dimensional array describing a players view of the field using the provided rectangle as a view
// port. The returned array is in row-major order.
func (m *MineField) ExtractPlayerView(viewport image.Rectangle) ViewPort {
	res := NewViewPort(viewport)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for y := viewport.Min.Y; y < viewport.Max.Y; y++ {
		// Translate viewport y to array index
		ay := y - viewport.Min.Y
		for x := viewport.Min.X; x < viewport.Max.X; x++ {
			// Translate viewport x to array index
			ax := x - viewport.Min.X

			pt := image.Pt(x, y)
			if m.IsMineOnLocation(x, y) && m.Triggered[pt] {
				res.Data[ay][ax] = VPEMine
			} else if m.Marks[pt] != MarkNone {
				if m.Marks[pt] == MarkQuestion {
					res.Data[ay][ax] = VPEMaybe
				} else {
					res.Data[ay][ax] = VPEFlag
				}
			} else {
				res.Data[ay][ax] = VPENone
			}

			mines, ok := m.Uncovered[image.Pt(x, y)]
			if ok {
				res.Data[ay][ax] = ViewPortElement('0' + mines)
			}
		}
	}

	return res
}

// CountNeighboringMines returns the number of mines bordering on the field identified by x, y
func (m *MineField) CountNeighboringMines(x int, y int) int {
	mines := 0
	for _, n := range m.Neighbors(image.Pt(x, y)) {
		if m.IsMineOnLocation(n.X, n.Y) {
			mines++
		}
	}
	return mines
}

// Mark cycles the mark at location x, y between "None", "Questionable", "Flagged"
//
// It locks m for writing
func (m *MineField) Mark(x int, y int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Only set marks on fields that have not been uncovered or triggered
	pt := image.Pt(x, y)
	m.Marks[pt] = (m.Marks[pt] + 1) % MarkMax
}

type UncoverResult int

const (
	UncoverMiss = iota
	UncoverBoom
)

// Uncover reveals the field at location x, y. It returns an UncoverResult that indicates whether an explosion was triggered and an
// integer count of the number of fields that were revealed.
//
// If the uncovered field has no neighboring mines, it uses a flood-fill algorithm to uncover neighboring cells until a "border" of
// mines is reached, or until the newly uncovered field is more than 30 fields distant from (x, y).
//
// Uncover locks m for writing.
func (m *MineField) Uncover(x int, y int) (UncoverResult, uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	point := image.Pt(x, y)

	// Don't do anything if the location has already been uncovered. Marks are ignored.
	_, isUncovered := m.Uncovered[point]
	if m.Triggered[point] || isUncovered {
		log.Printf("not doing anything for %s", point)
		return UncoverMiss, 0
	}

	// Remove location from list of marked points
	delete(m.Marks, point)

	// Count of uncovered fields
	numUncovered := uint(1)

	// Handle click:
	// - click on mine: game over
	// - click on empty field: count mines in 8 neighboring fields, set value
	if m.IsMineOnLocation(x, y) {
		m.Triggered[point] = true
		log.Println("BOOM", x, y)
		return UncoverBoom, numUncovered
	}

	mines := m.CountNeighboringMines(x, y)

	// If there are no mines in the vicinity, uncover fields until a "border" of mines is reached.
	if mines == 0 {
		numUncovered += m.FloodFill(x, y)
	}

	log.Printf("neighboring mines for x=%d, y=%d: %d", x, y, mines)
	m.Uncovered[point] = mines

	return UncoverMiss, numUncovered
}

// Neighbors returns the 8 points around p.
func (m *MineField) Neighbors(p image.Point) [8]image.Point {
	var (
		res [8]image.Point
		idx int
	)

	x, y := p.X, p.Y

	// Add all neighbors with distance less than maxRadius to the new unhandled set
	for xoff := -1; xoff <= 1; xoff++ {
		res[idx] = image.Pt(x+xoff, y-1)
		res[idx+1] = image.Pt(x+xoff, y+1)
		idx += 2
	}

	res[idx] = image.Pt(x-1, y)
	res[idx+1] = image.Pt(x+1, y)

	return res
}

// FloodFill starts a flood filling operation centered on x and y, uncovering fields without mines for a limited radius
func (m *MineField) FloodFill(x int, y int) uint {
	const maxRadius = 30 // Maximum uncovering distance

	center := image.Pt(x, y)
	dist := func(p image.Point) float64 {
		// Distance of p from center
		d := math.Sqrt(math.Pow(float64(center.X-p.X), 2) + math.Pow(float64(center.Y-p.Y), 2))
		return d
	}

	var numUncovered uint
	alreadyHandled := make(map[image.Point]bool)
	uncovered := make(map[image.Point]int)
	unhandled := make(map[image.Point]bool)
	unhandled[center] = true

	log.Printf("uncovering neighbors of (%d, %d)", x, y)

	for len(unhandled) != 0 {
		// As long as there are points to handle, keep uncovering
		newUnhandled := make(map[image.Point]bool)

		for pt := range unhandled {
			// If pt has 0 neighboring mines, add it to the uncovered set and add all its neighbors with distance less than maxRadius
			// to the new unhandled set
			mines := m.CountNeighboringMines(pt.X, pt.Y)
			uncovered[pt] = mines

			if mines != 0 {
				continue
			}

			for _, n := range m.Neighbors(pt) {
				if !alreadyHandled[n] && dist(n) <= maxRadius {
					newUnhandled[n] = true
				}
			}

			alreadyHandled[pt] = true
		}

		unhandled = newUnhandled
	}

	// Mark all uncovered on the minefield
	for pt, mines := range uncovered {
		_, alreadyUncovered := m.Uncovered[pt]
		if alreadyUncovered {
			// Already uncovered by someone else
			continue
		}
		numUncovered++
		m.Uncovered[pt] = mines
	}

	return numUncovered
}

const _zoom = 32

// RenderToImage returns a gray scale image that represents the are of the mine field m as indicated by the rectangle. The returned
// image is zoomed by a factor of 32. That is, the image is four times as wide and four times as high as rect.
func (m *MineField) RenderToImage(rect image.Rectangle) image.Image {
	img := image.NewGray(image.Rect(rect.Min.X*_zoom, rect.Min.Y*_zoom, rect.Max.X*_zoom, rect.Max.Y*_zoom))
	grid := image.Uniform{color.Black}

	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.ZP, draw.Over)

	for y := rect.Min.Y * _zoom; y < rect.Max.Y*_zoom; y += _zoom {
		r := image.Rect(rect.Min.X*_zoom, y-1, rect.Max.X*_zoom, y+1)
		draw.Draw(img, r, &grid, image.ZP, draw.Over)
	}

	for x := rect.Min.X * _zoom; x < rect.Max.X*_zoom; x += _zoom {
		r := image.Rect(x-1, rect.Min.Y*_zoom, x+1, rect.Max.Y*_zoom)
		draw.Draw(img, r, &grid, image.ZP, draw.Over)
	}

	mine := image.Uniform{color.Black}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if m.IsMineOnLocation(x, y) {
				x0 := x * _zoom
				y0 := y * _zoom
				x1 := (x + 1) * _zoom
				y1 := (y + 1) * _zoom
				r := image.Rect(x0, y0, x1, y1)
				draw.Draw(img, r, &mine, image.ZP, draw.Over)
			}
		}
	}

	return img
}
