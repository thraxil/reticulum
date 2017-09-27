// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package resize

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"strconv"
)

type SizeSpec struct {
	width  int
	height int
	square bool
	full   bool
}

// sizes are specified with a short string that can look like
//   full - full size, will not scale the image
//   100s - make a 100 pixel square image
//   200w - will make the image 200 pixels wide, preserving original aspect ratio
//   100h - will make it 100 pixels high, preserving original aspect ratio
//   100h300w - will make it 100 pixels high
//              and 300 wide (width and height can be specified in either order)
//
// images will always be cropped to match the desired aspect ratio rather than
// squished, cropping will always be centered.
//
// if 'full' or 's' are specified, they will take precedent over
// width and height specs.
//
// see Test_MakeSizeSpec in resize_test.go for more examples

func MakeSizeSpec(str string) *SizeSpec {
	s := SizeSpec{}
	if str == "full" {
		s.full = true
		s.width = -1
		s.height = -1
		return &s
	}
	r, _ := regexp.Compile("\\d+s")
	if m := r.FindString(str); m != "" {
		w, _ := strconv.Atoi(m[:len(m)-1])
		s.width = w
		s.height = w
		s.square = true
		return &s
	}
	// not full size or square, so we need to parse individual dimensions
	s.square = false
	s.full = false

	r, _ = regexp.Compile("\\d+w")
	if m := r.FindString(str); m != "" {
		w, _ := strconv.Atoi(m[:len(m)-1])
		s.width = w
	} else {
		// width was not set
		s.width = -1
	}
	r, _ = regexp.Compile("\\d+h")
	if m := r.FindString(str); m != "" {
		h, _ := strconv.Atoi(m[:len(m)-1])
		s.height = h
	} else {
		// height was not set
		s.height = -1
	}

	return &s
}

func (self SizeSpec) IsSquare() bool {
	return self.square
}

func (self SizeSpec) IsPortrait() bool {
	return self.height > self.width
}

func (self SizeSpec) IsLandscape() bool {
	return self.width > self.height
}

func (self SizeSpec) ToImageMagickSpec() string {
	if self.IsSquare() {
		return fmt.Sprintf("%dx%d^", self.width, self.width)
	}
	if self.width == -1 {
		return fmt.Sprintf("x%d", self.height)
	}
	if self.height == -1 {
		return fmt.Sprintf("%d", self.width)
	}
	return fmt.Sprintf("%dx%d", self.width, self.height)
}

func (self SizeSpec) String() string {
	if self.IsFull() {
		return "full"
	}
	if self.IsSquare() {
		return fmt.Sprintf("%ds", self.width)
	}
	if self.width == -1 {
		return fmt.Sprintf("%dh", self.height)
	}
	if self.height == -1 {
		return fmt.Sprintf("%dw", self.width)
	}
	return fmt.Sprintf("%dw%dh", self.width, self.height)
}

func (self SizeSpec) IsFull() bool {
	return self.full
}

func (self SizeSpec) Width() int {
	return self.width
}

func (self SizeSpec) Height() int {
	return self.height
}

func (self SizeSpec) MaxDimension() int {
	if self.width > self.height {
		return self.width
	}
	return self.height
}

func (self SizeSpec) MinDimension() int {
	if self.width < self.height {
		return self.width
	}
	return self.height
}

func rectMaxDimension(r image.Rectangle) int {
	if r.Dx() > r.Dy() {
		return r.Dx()
	}
	return r.Dy()
}

func rectMinDimension(r image.Rectangle) int {
	if r.Dx() < r.Dy() {
		return r.Dx()
	}
	return r.Dy()
}

func rectIsPortrait(r image.Rectangle) bool {
	return r.Dy() > r.Dx()
}

func rectIsLandscape(r image.Rectangle) bool {
	return r.Dx() > r.Dy()
}

func rectIsSquare(r image.Rectangle) bool {
	return r.Dx() == r.Dy()
}

// given an image size (as image.Rect), we match it up
// to the SizeSpec and return a new image.Rect which is
// essentially, the dimensions to crop the image to before scaling

func (self *SizeSpec) ToRect(rect image.Rectangle) image.Rectangle {
	if self.full || self.Width() == -1 || self.Height() == -1 {
		// full-size or only scaling one dimension, means we deal with the whole thing
		return rect
	}

	if self.square {
		if rectIsSquare(rect) {
			// already square. WIN.
			return rect
		}
		if rectIsLandscape(rect) {
			// crop and center on width
			trim := (rect.Dx() - rect.Dy()) / 2
			return image.Rect(trim, 0, rect.Dx()-trim, rect.Dy())
		} else {
			// portrait
			// crop and center on height
			trim := (rect.Dy() - rect.Dx()) / 2
			return image.Rect(0, trim, rect.Dx(), rect.Dy()-trim)
		}
	}
	if self.width == self.height {
		// "square" but not square.
		// fit it in a box with a max dimension, but don't crop
		// or scale up
		// in other words, return the whole thing. TargetWH will have to deal.
		return rect
	}
	// scaling both width and height
	if self.IsLandscape() {
		if rectIsSquare(rect) {
			// keep width, trim height
			ratio := float64(self.Height()) / float64(self.Width())
			targetHeight := int(ratio * float64(rect.Dx()))
			trim := targetHeight / 2
			return image.Rect(0, trim, rect.Dx(), rect.Dy()-trim)
		} else {
			if rectIsLandscape(rect) {
				ratio := float64(rect.Dy()) / float64(rect.Dx())
				outRatio := float64(self.Height()) / float64(self.Width())
				if ratio == outRatio {
					return rect
				}
				if outRatio > ratio {
					// want it even wider, so crop height
				} else {
					// narrower, so crop on width
				}
				rHeight := int(float64(rect.Dy()) * ratio)
				trim := (rect.Dy() - rHeight) / 2
				return image.Rect(0, trim, rect.Dx(), rect.Dy()-trim)
			} else {
				// rect.Dy() is the keeper
				ratio := float64(rect.Dx()) / float64(self.Width())
				rHeight := ratio * float64(self.Height())
				trim := int((float64(rect.Dy()) - rHeight) / 2)
				return image.Rect(0, trim, rect.Dx(), rect.Dy()-trim)
			}
		}
	} else {
		// portrait constraint
		if rectIsSquare(rect) {
			// keep height, trim width
			ratio := float64(self.Width()) / float64(self.Height())
			targetWidth := int(ratio * float64(rect.Dy()))
			trim := (rect.Dx() - targetWidth) / 2
			return image.Rect(trim, 0, rect.Dx()-trim, rect.Dx())
		} else {
			if rectIsLandscape(rect) {
				ratio := float64(self.Width()) / float64(self.Height())
				targetWidth := int(ratio * float64(rect.Dy()))
				trim := (rect.Dx() - targetWidth) / 2
				return image.Rect(trim, 0, rect.Dx()-trim, rect.Dy())
			} else {
				//rect.Dy() is the keeper
				ratio := float64(rect.Dx()) / float64(self.Width())
				rHeight := ratio * float64(self.Height())
				trim := int((float64(rect.Dy()) - rHeight) / 2)
				return image.Rect(0, trim, rect.Dx(), rect.Dy()-trim)
			}
		}

	}
	return rect
}

// size of the image that will result from resizing one of the
// specified rect to this SizeSpec
func (self *SizeSpec) TargetWH(rect image.Rectangle) (int, int) {
	if self.full {
		return rect.Dx(), rect.Dy()
	}
	if self.square {
		return self.width, self.height
	}
	if self.width == -1 {
		// scaling height only
		ratio := float64(rect.Dy()) / float64(self.height)
		x := int(float64(rect.Dx()) / ratio)
		return x, self.height
	}
	if self.height == -1 {
		// scaling height only
		ratio := float64(rect.Dx()) / float64(self.width)
		x := int(float64(rect.Dy()) / ratio)
		return self.width, x
	}

	return self.width, self.height
}

// Resize returns a scaled copy of the image slice r of m.
// The returned image has width w and height h.
func Resize(m image.Image, sizeStr string) image.Image {
	var w, h int

	ss := MakeSizeSpec(sizeStr)
	r := ss.ToRect(m.Bounds())
	w, h = ss.TargetWH(m.Bounds())

	if w < 0 || h < 0 {
		return nil
	}
	if w == 0 || h == 0 || r.Dx() <= 0 || r.Dy() <= 0 {
		return image.NewRGBA64(r)
	}

	switch m := m.(type) {
	case *image.RGBA:
		return resizeRGBA(m, r, w, h)
	}
	ww, hh := uint64(w), uint64(h)
	dx, dy := uint64(r.Dx()), uint64(r.Dy())
	// The scaling algorithm is to nearest-neighbor magnify the dx * dy source
	// to a (ww*dx) * (hh*dy) intermediate image and then minify the intermediate
	// image back down to a ww * hh destination with a simple box filter.
	// The intermediate image is implied, we do not physically allocate a slice
	// of length ww*dx*hh*dy.
	// For example, consider a 4*3 source image. Label its pixels from a-l:
	//	abcd
	//	efgh
	//	ijkl
	// To resize this to a 3*2 destination image, the intermediate is 12*6.
	// Whitespace has been added to delineate the destination pixels:
	//	aaab bbcc cddd
	//	aaab bbcc cddd
	//	eeef ffgg ghhh
	//
	//	eeef ffgg ghhh
	//	iiij jjkk klll
	//	iiij jjkk klll
	// Thus, the 'b' source pixel contributes one third of its value to the
	// (0, 0) destination pixel and two thirds to (1, 0).
	// The implementation is a two-step process. First, the source pixels are
	// iterated over and each source pixel's contribution to 1 or more
	// destination pixels are summed. Second, the sums are divided by a scaling
	// factor to yield the destination pixels.
	// TODO: By interleaving the two steps, instead of doing all of
	// step 1 first and all of step 2 second, we could allocate a smaller sum
	// slice of length 4*w*2 instead of 4*w*h, although the resultant code
	// would become more complicated.
	n, sum := dx*dy, make([]uint64, 4*w*h)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			// Get the source pixel.
			r32, g32, b32, a32 := m.At(x, y).RGBA()
			r64 := uint64(r32)
			g64 := uint64(g32)
			b64 := uint64(b32)
			a64 := uint64(a32)
			// Spread the source pixel over 1 or more destination rows.
			py := uint64(y-r.Min.Y) * hh
			for remy := hh; remy > 0; {
				qy := dy - (py % dy)
				if qy > remy {
					qy = remy
				}
				// Spread the source pixel over 1 or more destination columns.
				px := uint64(x-r.Min.X) * ww
				index := 4 * ((py/dy)*ww + (px / dx))
				for remx := ww; remx > 0; {
					qx := dx - (px % dx)
					if qx > remx {
						qx = remx
					}
					sum[index+0] += r64 * qx * qy
					sum[index+1] += g64 * qx * qy
					sum[index+2] += b64 * qx * qy
					sum[index+3] += a64 * qx * qy
					index += 4
					px += qx
					remx -= qx
				}
				py += qy
				remy -= qy
			}
		}
	}
	return average(sum, w, h, n*0x0101)
}

// average convert the sums to averages and returns the result.
func average(sum []uint64, w, h int, n uint64) image.Image {
	r := image.Rect(0, 0, w, h)
	ret := image.NewRGBA(r)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*ret.Stride + x*4
			j := 4 * (y*w + x)
			ret.Pix[i+0] = uint8(sum[j+0] / n)
			ret.Pix[i+1] = uint8(sum[j+1] / n)
			ret.Pix[i+2] = uint8(sum[j+2] / n)
			ret.Pix[i+3] = uint8(sum[j+3] / n)
		}
	}
	return ret
}

// resizeRGBA returns a scaled copy of the RGBA image slice r of m.
// The returned image has width w and height h.
func resizeRGBA(m *image.RGBA, r image.Rectangle, w, h int) image.Image {
	ww, hh := uint64(w), uint64(h)
	dx, dy := uint64(r.Dx()), uint64(r.Dy())
	// See comment in Resize.
	n, sum := dx*dy, make([]uint64, 4*w*h)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		pix := m.Pix[(y-m.Rect.Min.Y)*m.Stride:]
		for x := r.Min.X; x < r.Max.X; x++ {
			// Get the source pixel.
			p := pix[(x-m.Rect.Min.X)*4:]
			r64 := uint64(p[0])
			g64 := uint64(p[1])
			b64 := uint64(p[2])
			a64 := uint64(p[3])
			// Spread the source pixel over 1 or more destination rows.
			py := uint64(y-r.Min.Y) * hh
			for remy := hh; remy > 0; {
				qy := dy - (py % dy)
				if qy > remy {
					qy = remy
				}
				// Spread the source pixel over 1 or more destination columns.
				px := uint64(x-r.Min.X) * ww
				index := 4 * ((py/dy)*ww + (px / dx))
				for remx := ww; remx > 0; {
					qx := dx - (px % dx)
					if qx > remx {
						qx = remx
					}
					qxy := qx * qy
					sum[index+0] += r64 * qxy
					sum[index+1] += g64 * qxy
					sum[index+2] += b64 * qxy
					sum[index+3] += a64 * qxy
					index += 4
					px += qx
					remx -= qx
				}
				py += qy
				remy -= qy
			}
		}
	}
	return average(sum, w, h, n)
}

// Resample returns a resampled copy of the image slice r of m.
// The returned image has width w and height h.
// plain old Nearest Neighbor algorithm
func Resample(m image.Image, r image.Rectangle, w, h int) image.Image {
	if w < 0 || h < 0 {
		return nil
	}
	if w == 0 || h == 0 || r.Dx() <= 0 || r.Dy() <= 0 {
		return image.NewRGBA64(r)
	}
	curw, curh := r.Dx(), r.Dy()
	img := image.NewRGBA(r)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Get a source pixel.
			subx := x * curw / w
			suby := y * curh / h
			r32, g32, b32, a32 := m.At(subx, suby).RGBA()
			r := uint8(r32 >> 8)
			g := uint8(g32 >> 8)
			b := uint8(b32 >> 8)
			a := uint8(a32 >> 8)
			img.SetRGBA(x, y, color.RGBA{r, g, b, a})
		}
	}
	return img
}
