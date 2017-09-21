package main

import (
	_ "image/png"
	_ "image/jpeg"
	"os"
	"image"
	"image/color"
	"log"
	"time"
	"fmt"
	tri "github.com/esimov/triangle"
	"github.com/fogleman/gg"
	"flag"
)

func main() {
	var (
		// Flags
		source		= flag.String("in", "", "Source")
		destination	= flag.String("out", "", "Destination")
		blurRadius	= flag.Int("blur", 4, "Blur radius")
		sobelThreshold	= flag.Int("sobel", 10, "Sobel filter threshold")
		pointsThreshold	= flag.Int("points", 20, "Points threshold")
		maxPoints	= flag.Int("max", 2500, "Maximum number of points")
		wireframe	= flag.Bool("wireframe", false, "Wireframe mode")

		blur, gray, sobel *image.NRGBA
		triangles 	[]tri.Triangle
		points 		[]tri.Point
	)

	flag.Parse()

	file, err := os.Open(*source)
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		panic(err)
	}

	width, height := src.Bounds().Dx(), src.Bounds().Dy()
	ctx := gg.NewContext(width, height)
	ctx.DrawRectangle(0, 0, float64(width), float64(height))
	ctx.SetRGBA(1, 1, 1, 1)
	ctx.Fill()

	delaunay := &tri.Delaunay{}
	img := toNRGBA(src)

	start := time.Now()
	spinner("Generating triangulated image...")

	blur = tri.Stackblur(img, uint32(width), uint32(height), uint32(*blurRadius))
	gray = tri.Grayscale(blur)
	sobel = tri.SobelFilter(gray, float64(*sobelThreshold))
	points = tri.GetEdgePoints(sobel, *pointsThreshold, *maxPoints)

	triangles = delaunay.Init(width, height).Insert(points).GetTriangles()

	for i := 0; i < len(triangles); i++ {
		t := triangles[i]
		p0, p1, p2 := t.Nodes[0], t.Nodes[1], t.Nodes[2]

		ctx.Push()
		ctx.MoveTo(float64(p0.X), float64(p0.Y))
		ctx.LineTo(float64(p1.X), float64(p1.Y))
		ctx.LineTo(float64(p2.X), float64(p2.Y))
		ctx.LineTo(float64(p0.X), float64(p0.Y))

		cx := float64(p0.X + p1.X + p2.X) * 0.33333
		cy := float64(p0.Y + p1.Y + p2.Y) * 0.33333

		j := ((int(cx) | 0) + (int(cy) | 0) * width) * 4
		r, g, b := img.Pix[j], img.Pix[j + 1], img.Pix[j + 2]

		ctx.SetLineWidth(2)
		ctx.SetFillStyle(gg.NewSolidPattern(color.RGBA{R:r, G:g, B:b, A:255}))
		ctx.SetStrokeStyle(gg.NewSolidPattern(color.RGBA{R:r, G:g, B:b, A:255}))

		if !*wireframe {
			ctx.StrokePreserve()
			ctx.FillPreserve()
			ctx.Fill()
		}
		ctx.Stroke()
		ctx.Pop()
	}

	if err = ctx.SavePNG(*destination); err != nil {
		log.Fatal(err)
	}

	end := time.Since(start)
	fmt.Printf("\nGenerated in: %.2fs\n", end.Seconds())
	fmt.Printf("Total number of %d triangles generated out of %d points\n", len(triangles), len(points))
}

// toNRGBA converts any image type to *image.NRGBA with min-point at (0, 0).
func toNRGBA(img image.Image) *image.NRGBA {
	srcBounds := img.Bounds()
	if srcBounds.Min.X == 0 && srcBounds.Min.Y == 0 {
		if src0, ok := img.(*image.NRGBA); ok {
			return src0
		}
	}
	srcMinX := srcBounds.Min.X
	srcMinY := srcBounds.Min.Y

	dstBounds := srcBounds.Sub(srcBounds.Min)
	dstW := dstBounds.Dx()
	dstH := dstBounds.Dy()
	dst := image.NewNRGBA(dstBounds)

	switch src := img.(type) {
	case *image.NRGBA:
		rowSize := srcBounds.Dx() * 4
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			si := src.PixOffset(srcMinX, srcMinY+dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				copy(dst.Pix[di:di+rowSize], src.Pix[si:si+rowSize])
			}
		}
	case *image.YCbCr:
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				srcX := srcMinX + dstX
				srcY := srcMinY + dstY
				siy := src.YOffset(srcX, srcY)
				sic := src.COffset(srcX, srcY)
				r, g, b := color.YCbCrToRGB(src.Y[siy], src.Cb[sic], src.Cr[sic])
				dst.Pix[di+0] = r
				dst.Pix[di+1] = g
				dst.Pix[di+2] = b
				dst.Pix[di+3] = 0xff
				di += 4
			}
		}
	case *image.Gray:
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			si := src.PixOffset(srcMinX, srcMinY+dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				c := src.Pix[si]
				dst.Pix[di+0] = c
				dst.Pix[di+1] = c
				dst.Pix[di+2] = c
				dst.Pix[di+3] = 0xff
				di += 4
				si += 2
			}
		}
	default:
		for dstY := 0; dstY < dstH; dstY++ {
			di := dst.PixOffset(0, dstY)
			for dstX := 0; dstX < dstW; dstX++ {
				c := color.NRGBAModel.Convert(img.At(srcMinX+dstX, srcMinY+dstY)).(color.NRGBA)
				dst.Pix[di+0] = c.R
				dst.Pix[di+1] = c.G
				dst.Pix[di+2] = c.B
				dst.Pix[di+3] = c.A
				di += 4
			}
		}
	}

	return dst
}

// Function to visualize the rendering progress
func spinner(message string) {
	go func() {
		for {
			for _, r := range `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` {
				fmt.Printf("\r%s%s %c%s", message, "\x1b[92m", r, "\x1b[39m")
				time.Sleep(time.Millisecond * 100)
			}
		}
	}()
}