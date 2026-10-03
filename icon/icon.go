// Package icon rasterizes the tray icon — a Kubernetes-style helm
// wheel on a heptagon — into the ARGB32 pixmaps StatusNotifierItem
// expects. Icon themes don't ship a kubernetes icon, and drawing it in
// code avoids bundling image files or pulling in an image library.
package icon

import (
	"math"

	"k8s-context-switcher/tray"
)

// Color is an opaque RGB color for the heptagon background.
type Color struct{ R, G, B uint8 }

var (
	// Blue is the Kubernetes brand blue, used for regular contexts.
	Blue = Color{0x32, 0x6C, 0xE5}
	// Red flags the current context as production.
	Red = Color{0xD9, 0x30, 0x25}
)

// Sizes rendered for the host to pick from; hosts scale the closest one.
var Sizes = []int{16, 22, 24, 32, 48, 64}

// Render draws the icon at every size in Sizes.
func Render(bg Color) []tray.Pixmap {
	out := make([]tray.Pixmap, 0, len(Sizes))
	for _, s := range Sizes {
		out = append(out, renderSize(s, bg))
	}
	return out
}

const (
	sides      = 7
	outerR     = 0.98 // heptagon circumradius, in [-1,1] icon space
	ringInner  = 0.40
	ringOuter  = 0.54
	hubR       = 0.16
	spokeLen   = 0.74
	spokeHalfW = 0.075
	supersamp  = 4 // per axis, for antialiasing
)

// vertexAngle is the angle of heptagon vertex k, with vertex 0 pointing up.
func vertexAngle(k int) float64 {
	return -math.Pi/2 + 2*math.Pi*float64(k)/sides
}

func inHeptagon(x, y float64) bool {
	apothem := outerR * math.Cos(math.Pi/sides)
	for k := 0; k < sides; k++ {
		a := vertexAngle(k) + math.Pi/sides // outward normal of edge k
		if x*math.Cos(a)+y*math.Sin(a) > apothem {
			return false
		}
	}
	return true
}

func inWheel(x, y float64) bool {
	r := math.Hypot(x, y)
	if r <= hubR || (r >= ringInner && r <= ringOuter) {
		return true
	}
	// Spokes run from the hub out past the ring toward each vertex,
	// with a round cap at the tip.
	for k := 0; k < sides; k++ {
		a := vertexAngle(k)
		dx, dy := math.Cos(a), math.Sin(a)
		t := x*dx + y*dy
		if t < 0 {
			continue
		}
		if t > spokeLen {
			t = spokeLen
		}
		if math.Hypot(x-t*dx, y-t*dy) <= spokeHalfW {
			return true
		}
	}
	return false
}

func renderSize(size int, bg Color) tray.Pixmap {
	data := make([]byte, 0, size*size*4)
	n := float64(supersamp * supersamp)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var bgHits, fgHits float64
			for sy := 0; sy < supersamp; sy++ {
				for sx := 0; sx < supersamp; sx++ {
					x := (float64(px)+(float64(sx)+0.5)/supersamp)/float64(size)*2 - 1
					y := (float64(py)+(float64(sy)+0.5)/supersamp)/float64(size)*2 - 1
					if !inHeptagon(x, y) {
						continue
					}
					if inWheel(x, y) {
						fgHits++
					} else {
						bgHits++
					}
				}
			}
			covered := bgHits + fgHits
			if covered == 0 {
				data = append(data, 0, 0, 0, 0)
				continue
			}
			// Straight (non-premultiplied) alpha: color is the mix of
			// covered samples, alpha is the covered fraction.
			mix := func(c uint8) byte {
				return byte((float64(c)*bgHits + 255*fgHits) / covered)
			}
			data = append(data, byte(255*covered/n), mix(bg.R), mix(bg.G), mix(bg.B))
		}
	}
	return tray.Pixmap{Width: int32(size), Height: int32(size), Data: data}
}
