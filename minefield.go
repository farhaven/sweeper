package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"math/rand"
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

const _zoom = 32

// IntToBytes converts x to a little endian byte slice
func IntToBytes(val int64) []byte {
	x := uint64(val + math.MinInt64)
	res := [8]byte{}

	for idx := 0; idx < 8; idx++ {
		res[idx] = byte(x >> (8 * idx))
	}

	return res[:]
}

type MineField struct {
	sync.RWMutex
	seed      [16]byte
	threshold uint32
	// Map of coordinates to neighboring mine count
	uncovered map[image.Point]int
	// Map of triggered mines
	triggered map[image.Point]bool
}

func NewMineField(threshold uint32) (*MineField, error) {
	m := MineField{
		threshold: 5,
		uncovered: make(map[image.Point]int),
		triggered: make(map[image.Point]bool),
	}
	_, err := rand.Read(m.seed[:])
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// IsMineOnLocation returns true if there is a mine in the location indicated by x and y.
func (m *MineField) IsMineOnLocation(x, y int) bool {
	// Determine wether x, y contains a mine by hashing it with the seed and checking whether it's less than a threshold
	h := fnv.New32()
	h.Write(m.seed[:])
	h.Write(IntToBytes(int64(x)))
	h.Write(IntToBytes(int64(y)))
	return (h.Sum32() % m.threshold) == 0
}

// ExtractPlayerView returns a 2 dimensional array describing a players view of the field using the provided rectangle as a view
// port. The returned array is in row-major order.
func (m *MineField) ExtractPlayerView(viewport image.Rectangle) ViewPort {
	res := NewViewPort(viewport)

	m.RLock()
	defer m.RUnlock()

	for y := viewport.Min.Y; y < viewport.Max.Y; y++ {
		// Translate viewport y to array index
		ay := y - viewport.Min.Y
		for x := viewport.Min.X; x < viewport.Max.X; x++ {
			// Translate viewport x to array index
			ax := x - viewport.Min.X

			if m.IsMineOnLocation(x, y) && m.triggered[image.Pt(x, y)] {
				res.Data[ay][ax] = VPEMine
			} else {
				res.Data[ay][ax] = VPENone
			}

			mines, ok := m.uncovered[image.Pt(x, y)]
			if ok {
				res.Data[ay][ax] = ViewPortElement('0' + mines)
			}
		}
	}

	return res
}

func (m *MineField) HandleClick(x int, y int) {
	m.Lock()
	defer m.Unlock()

	point := image.Pt(x, y)

	// Don't do anything if the location has already been clicked on
	_, uncovered := m.uncovered[point]
	if m.triggered[point] || uncovered {
		log.Printf("not doing anything for %s", point)
		return
	}

	// Handle click:
	// - click on mine: game over
	// - click on empty field: count mines in 8 neighboring fields, set value
	if m.IsMineOnLocation(x, y) {
		m.triggered[point] = true
		log.Println("BOOM", x, y)
		return
	}

	// count mines in neighboring fields
	mines := 0
	for xoff := -1; xoff <= 1; xoff++ {
		if m.IsMineOnLocation(x+xoff, y-1) {
			mines++
		}
		if m.IsMineOnLocation(x+xoff, y+1) {
			mines++
		}
	}

	if m.IsMineOnLocation(x-1, y) {
		mines++
	}
	if m.IsMineOnLocation(x+1, y) {
		mines++
	}

	log.Printf("neighboring mines for x=%d, y=%d: %d", x, y, mines)
	m.uncovered[point] = mines
}

// RenderToImage returns a gray scale image that represents the are of the mine field m as indicated by the rectangle. The returned
// image is zoomed by a factor of 4. That is, the image is four times as wide and four times as high as rect.
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
