package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
)

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
	seed      [16]byte
	threshold uint32
}

func NewMineField(threshold uint32) (MineField, error) {
	m := MineField{
		threshold: 5,
	}
	_, err := rand.Read(m.seed[:])
	if err != nil {
		return m, err
	}
	return m, nil
}

// IsMineOnLocation returns true if there is a mine in the location indicated by x and y.
func (m MineField) IsMineOnLocation(x, y int) bool {
	// Determine wether x, y contains a mine by hashing it with the seed and checking whether it's less than a threshold
	h := fnv.New32()
	h.Write(m.seed[:])
	h.Write(IntToBytes(int64(x)))
	h.Write(IntToBytes(int64(y)))
	return (h.Sum32() % m.threshold) == 0
}

// RenderToImage returns a gray scale image that represents the are of the mine field m as indicated by the rectangle. The returned
// image is zoomed by a factor of 4. That is, the image is four times as wide and four times as high as rect.
func (m MineField) RenderToImage(rect image.Rectangle) image.Image {
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

func (m MineField) clickHandler(w http.ResponseWriter, r *http.Request) {
	x, err := strconv.Atoi(r.FormValue("x"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid x: %s\n", err)
		return
	}

	y, err := strconv.Atoi(r.FormValue("y"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid x: %s\n", err)
		return
	}

	x /= _zoom
	y /= _zoom

	log.Println("click on", x, y)
}

func (m MineField) fieldHandler(w http.ResponseWriter, r *http.Request) {
	badParam := func(name string, err error) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid %s: %s\n", name, err)
	}

	x0, err := strconv.Atoi(r.FormValue("x0"))
	if err != nil {
		badParam("x0", err)
		return
	}

	y0, err := strconv.Atoi(r.FormValue("y0"))
	if err != nil {
		badParam("y0", err)
		return
	}

	x1, err := strconv.Atoi(r.FormValue("x1"))
	if err != nil {
		badParam("x1", err)
		return
	}

	y1, err := strconv.Atoi(r.FormValue("y1"))
	if err != nil {
		badParam("y1", err)
		return
	}

	rect := image.Rect(x0, y0, x1, y1)

	log.Printf("got request for %s, dx=%d, dy=%d", rect, rect.Dx(), rect.Dy())

	w.Header().Add("content-type", "image/png")
	w.Header().Add("Cache-Control", "no-cache, private, max-age=0")
	w.WriteHeader(http.StatusOK)
	image := m.RenderToImage(rect)
	err = png.Encode(w, image)
	if err != nil {
		log.Printf("Can't render %s to png: %s", rect, err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	log.Println("got request for index")

	fh, err := os.Open("index.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "can't open index: %s", err)
		return
	}
	defer fh.Close()

	w.Header().Add("content-type", "text/html")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, fh)
}

/*
- Render field
- Build flood fill, up to a certain maximum radius
*/
func main() {
	log.Println("Here we go")

	m, err := NewMineField(4)
	if err != nil {
		log.Fatalln("can't create mine field:", err)
	}

	rect := image.Rect(-100, -100, 100, 100)
	img := m.RenderToImage(rect)

	w, err := os.Create("foo.png")
	if err != nil {
		log.Fatalln("can't open foo.png:", err)
	}
	defer w.Close()

	err = png.Encode(w, img)
	if err != nil {
		log.Fatalln("can't encode png:", err)
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/field", m.fieldHandler)
	http.HandleFunc("/click", m.clickHandler)

	log.Println("HTTP handler set up")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalln("can't run http server:", err)
	}
}
