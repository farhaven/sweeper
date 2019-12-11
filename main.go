package main

import (
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math/rand"
	"os"
)

// IntToBytes converts x to a little endian byte slice
func IntToBytes(x int64) []byte {
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
		threshold: 3,
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
	const zoom = 4

	img := image.NewGray(image.Rect(rect.Min.X*zoom, rect.Min.Y*zoom, rect.Max.X*zoom, rect.Max.Y*zoom))
	mine := image.Uniform{color.Black}

	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.ZP, draw.Over)

	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if m.IsMineOnLocation(x, y) {
				draw.Draw(img, image.Rect(x*zoom, y*zoom, (x+1)*zoom, (y+1)*zoom), &mine, image.ZP, draw.Over)
			}
		}
	}

	return img
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
}
