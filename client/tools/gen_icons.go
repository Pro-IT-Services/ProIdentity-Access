//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type pt struct{ x, y float64 }

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	writePNG(filepath.Join(root, "assets", "trayicon.png"), 1024)
	writePNG(filepath.Join(root, "build", "appicon.png"), 1024)
	writeICO(filepath.Join(root, "build", "windows", "icon.ico"), []int{16, 24, 32, 48, 64, 128, 256})
}

func writePNG(path string, size int) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, render(size)); err != nil {
		panic(err)
	}
}

func writeICO(path string, sizes []int) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	images := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(size)); err != nil {
			panic(err)
		}
		images = append(images, buf.Bytes())
	}

	var out bytes.Buffer
	write16(&out, 0)
	write16(&out, 1)
	write16(&out, uint16(len(sizes)))
	offset := 6 + len(sizes)*16
	for i, size := range sizes {
		if size >= 256 {
			out.WriteByte(0)
			out.WriteByte(0)
		} else {
			out.WriteByte(byte(size))
			out.WriteByte(byte(size))
		}
		out.WriteByte(0)
		out.WriteByte(0)
		write16(&out, 1)
		write16(&out, 32)
		write32(&out, uint32(len(images[i])))
		write32(&out, uint32(offset))
		offset += len(images[i])
	}
	for _, data := range images {
		out.Write(data)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		panic(err)
	}
}

func render(size int) image.Image {
	const ss = 4
	canvas := image.NewNRGBA(image.Rect(0, 0, size*ss, size*ss))
	s := float64(size * ss)

	bg := color.NRGBA{R: 8, G: 13, B: 24, A: 255}
	border := color.NRGBA{R: 255, G: 255, B: 255, A: 15}
	white := color.NRGBA{R: 230, G: 238, B: 250, A: 230}
	whiteSoft := color.NRGBA{R: 230, G: 238, B: 250, A: 54}
	blue := color.NRGBA{R: 59, G: 130, B: 246, A: 255}

	fillRoundedRect(canvas, 0, 0, s, s, s*0.21875, bg)
	strokeRoundedRect(canvas, 2*ss, 2*ss, s-2*ss, s-2*ss, s*0.214, float64(ss), border)

	cx, cy, r := s/2, s/2, s*100/256
	strokeCircle(canvas, cx, cy, r, float64(2*ss), whiteSoft)

	to := func(x, y float64) pt { return pt{x * s / 256, y * s / 256} }
	hex := []pt{to(178, 128), to(153, 171.3), to(103, 171.3), to(78, 128), to(103, 84.7), to(153, 84.7)}
	strokePoly(canvas, hex, float64(8*ss), white, true)
	lines := [][2]pt{
		{to(178, 128), to(192.2, 199.3)},
		{to(153, 171.3), to(98.3, 219.3)},
		{to(103, 171.3), to(34.1, 148.0)},
		{to(78, 128), to(63.8, 56.7)},
		{to(103, 84.7), to(157.7, 36.7)},
		{to(153, 84.7), to(221.9, 108.0)},
	}
	for _, l := range lines {
		strokeLine(canvas, l[0], l[1], float64(6*ss), white)
	}
	fillCircle(canvas, cx, cy, s*11/256, blue)

	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var rr, gg, bb, aa uint32
			for yy := 0; yy < ss; yy++ {
				for xx := 0; xx < ss; xx++ {
					c := canvas.NRGBAAt(x*ss+xx, y*ss+yy)
					rr += uint32(c.R)
					gg += uint32(c.G)
					bb += uint32(c.B)
					aa += uint32(c.A)
				}
			}
			n := uint32(ss * ss)
			dst.SetNRGBA(x, y, color.NRGBA{uint8(rr / n), uint8(gg / n), uint8(bb / n), uint8(aa / n)})
		}
	}
	return dst
}

func fillRoundedRect(img *image.NRGBA, x0, y0, x1, y1, r float64, c color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			xf, yf := float64(x)+0.5, float64(y)+0.5
			if inRoundedRect(xf, yf, x0, y0, x1, y1, r) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func strokeRoundedRect(img *image.NRGBA, x0, y0, x1, y1, r, w float64, c color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			xf, yf := float64(x)+0.5, float64(y)+0.5
			if inRoundedRect(xf, yf, x0, y0, x1, y1, r) && !inRoundedRect(xf, yf, x0+w, y0+w, x1-w, y1-w, math.Max(0, r-w)) {
				over(img, x, y, c)
			}
		}
	}
}

func inRoundedRect(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := math.Min(math.Max(x, x0+r), x1-r)
	cy := math.Min(math.Max(y, y0+r), y1-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

func strokeCircle(img *image.NRGBA, cx, cy, r, w float64, c color.NRGBA) {
	b := img.Bounds()
	outer := r + w/2
	inner := r - w/2
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			if d >= inner && d <= outer {
				over(img, x, y, c)
			}
		}
	}
}

func fillCircle(img *image.NRGBA, cx, cy, r float64, c color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= r {
				over(img, x, y, c)
			}
		}
	}
}

func strokePoly(img *image.NRGBA, points []pt, w float64, c color.NRGBA, closed bool) {
	for i := 0; i < len(points)-1; i++ {
		strokeLine(img, points[i], points[i+1], w, c)
	}
	if closed && len(points) > 1 {
		strokeLine(img, points[len(points)-1], points[0], w, c)
	}
}

func strokeLine(img *image.NRGBA, a, b pt, w float64, c color.NRGBA) {
	minX := int(math.Floor(math.Min(a.x, b.x) - w))
	maxX := int(math.Ceil(math.Max(a.x, b.x) + w))
	minY := int(math.Floor(math.Min(a.y, b.y) - w))
	maxY := int(math.Ceil(math.Max(a.y, b.y) + w))
	r := w / 2
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if image.Pt(x, y).In(img.Bounds()) && distToSegment(float64(x)+0.5, float64(y)+0.5, a, b) <= r {
				over(img, x, y, c)
			}
		}
	}
	fillCircle(img, a.x, a.y, r, c)
	fillCircle(img, b.x, b.y, r, c)
}

func distToSegment(x, y float64, a, b pt) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(x-a.x, y-a.y)
	}
	t := math.Max(0, math.Min(1, ((x-a.x)*dx+(y-a.y)*dy)/l2))
	px, py := a.x+t*dx, a.y+t*dy
	return math.Hypot(x-px, y-py)
}

func over(img *image.NRGBA, x, y int, c color.NRGBA) {
	dst := img.NRGBAAt(x, y)
	src := image.NewUniform(c)
	tmp := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	tmp.SetNRGBA(0, 0, dst)
	draw.Draw(tmp, tmp.Bounds(), src, image.Point{}, draw.Over)
	img.SetNRGBA(x, y, tmp.NRGBAAt(0, 0))
}

func write16(buf *bytes.Buffer, v uint16) { _ = binary.Write(buf, binary.LittleEndian, v) }
func write32(buf *bytes.Buffer, v uint32) { _ = binary.Write(buf, binary.LittleEndian, v) }
