package resize

import (
	"image"
	"testing"
)

type SizeSpecTestCase struct {
	SizeSpecString string
	Full           bool
	Square         bool
	ExpectedWidth  int
	ExpectedHeight int
}

func Test_MakeSizeSpec(t *testing.T) {
	cases := []SizeSpecTestCase{
		{
			SizeSpecString: "100s",
			Full:           false,
			Square:         true,
			ExpectedWidth:  100,
			ExpectedHeight: 100,
		},
		{
			SizeSpecString: "100w",
			Full:           false,
			Square:         false,
			ExpectedWidth:  100,
			ExpectedHeight: -1,
		},
		{
			SizeSpecString: "100h",
			Full:           false,
			Square:         false,
			ExpectedWidth:  -1,
			ExpectedHeight: 100,
		},
		{
			SizeSpecString: "100h200w",
			Full:           false,
			Square:         false,
			ExpectedWidth:  200,
			ExpectedHeight: 100,
		},
		{
			SizeSpecString: "200w100h",
			Full:           false,
			Square:         false,
			ExpectedWidth:  200,
			ExpectedHeight: 100,
		},
		{
			SizeSpecString: "100w200h",
			Full:           false,
			Square:         false,
			ExpectedWidth:  100,
			ExpectedHeight: 200,
		},
		{
			SizeSpecString: "200h100w",
			Full:           false,
			Square:         false,
			ExpectedWidth:  100,
			ExpectedHeight: 200,
		},
		{
			SizeSpecString: "full",
			Full:           true,
			Square:         false,
			ExpectedWidth:  -1,
			ExpectedHeight: -1,
		},
	}

	for i := range cases {
		c := cases[i]
		ss := MakeSizeSpec(c.SizeSpecString)
		if c.Full {
			if !ss.IsFull() {
				t.Error(c.SizeSpecString, "-- should be full-size but is not")
			}
		} else {
			if ss.IsFull() {
				t.Error(c.SizeSpecString, "-- should not be full-size but is")
			}
		}

		if c.Square {
			if !ss.IsSquare() {
				t.Error(c.SizeSpecString, "-- should be square but is not")
			}
		} else {
			if ss.IsSquare() {
				t.Error(c.SizeSpecString, "-- should not be square but is")
			}
		}

		if ss.Width() != c.ExpectedWidth {
			t.Error(c.SizeSpecString, "-- bad width", ss.Width(), "expected", c.ExpectedWidth)
		}
		if ss.Height() != c.ExpectedHeight {
			t.Error(c.SizeSpecString, "-- bad height", ss.Height(), "expected", c.ExpectedHeight)
		}
	}

}

type toRectTestCase struct {
	Label          string
	SizeSpec       *SizeSpec
	Rect           image.Rectangle
	ExpectedWidth  int
	ExpectedHeight int
}

func Test_ToRect(t *testing.T) {
	full_sized := MakeSizeSpec("full")
	square_sized := MakeSizeSpec("100s")
	width_constrained := MakeSizeSpec("100w")
	height_constrained := MakeSizeSpec("100h")
	box_constrained := MakeSizeSpec("100w100h")
	height_and_width_constrained_landscape := MakeSizeSpec("100w50h")
	height_and_width_constrained_portrait := MakeSizeSpec("100h50w")
	landscape := image.Rect(0, 0, 1000, 500)
	portrait := image.Rect(0, 0, 500, 1000)
	square_image := image.Rect(0, 0, 1000, 1000)

	cases := []toRectTestCase{
		{
			Label:          "full-sized should be no-op",
			SizeSpec:       full_sized,
			Rect:           square_image,
			ExpectedWidth:  square_image.Dx(),
			ExpectedHeight: square_image.Dy(),
		},

		{
			Label:          "box constraint on square (eq/sq)",
			SizeSpec:       box_constrained,
			Rect:           square_image,
			ExpectedWidth:  square_image.Dx(),
			ExpectedHeight: square_image.Dy(),
		},

		{
			Label:          "box constraint on portrait (eq/port)",
			SizeSpec:       box_constrained,
			Rect:           portrait,
			ExpectedWidth:  portrait.Dx(),
			ExpectedHeight: portrait.Dy(),
		},

		{
			Label:          "box constraint on landscape (eq/landscape)",
			SizeSpec:       box_constrained,
			Rect:           landscape,
			ExpectedWidth:  landscape.Dx(),
			ExpectedHeight: landscape.Dy(),
		},

		{
			Label:          "square == square",
			SizeSpec:       square_sized,
			Rect:           square_image,
			ExpectedWidth:  square_image.Dx(),
			ExpectedHeight: square_image.Dy(),
		},

		{
			Label:          "square != square (wider than taller)",
			SizeSpec:       square_sized,
			Rect:           landscape,
			ExpectedWidth:  500,
			ExpectedHeight: 500,
		},

		{
			Label:          "square != square (taller than wide)",
			SizeSpec:       square_sized,
			Rect:           portrait,
			ExpectedWidth:  500,
			ExpectedHeight: 500,
		},

		{
			Label:          "width constrained square image",
			SizeSpec:       width_constrained,
			Rect:           square_image,
			ExpectedWidth:  1000,
			ExpectedHeight: 1000,
		},

		{
			Label:          "width constrained wider than tall",
			SizeSpec:       width_constrained,
			Rect:           landscape,
			ExpectedWidth:  1000,
			ExpectedHeight: 500,
		},

		{
			Label:          "width constrained taller than wide",
			SizeSpec:       width_constrained,
			Rect:           portrait,
			ExpectedWidth:  500,
			ExpectedHeight: 1000,
		},

		{
			Label:          "height constrained square image",
			SizeSpec:       height_constrained,
			Rect:           square_image,
			ExpectedWidth:  1000,
			ExpectedHeight: 1000,
		},

		{
			Label:          "height constrained wider than tall",
			SizeSpec:       height_constrained,
			Rect:           landscape,
			ExpectedWidth:  1000,
			ExpectedHeight: 500,
		},

		{
			Label:          "height constrained taller than wide",
			SizeSpec:       height_constrained,
			Rect:           portrait,
			ExpectedWidth:  500,
			ExpectedHeight: 1000,
		},

		{
			Label:          "height and width constrained (landscape) square",
			SizeSpec:       height_and_width_constrained_landscape,
			Rect:           square_image,
			ExpectedWidth:  1000,
			ExpectedHeight: 500,
		},

		{
			Label:          "height and width constrained (landscape) portrait",
			SizeSpec:       height_and_width_constrained_landscape,
			Rect:           portrait,
			ExpectedWidth:  500,
			ExpectedHeight: 250,
		},

		{
			Label:          "height and width constrained (landscape) portrait",
			SizeSpec:       height_and_width_constrained_landscape,
			Rect:           landscape,
			ExpectedWidth:  1000,
			ExpectedHeight: 500,
		},

		{
			Label:          "height and width constrained (portrait) square",
			SizeSpec:       height_and_width_constrained_portrait,
			Rect:           square_image,
			ExpectedWidth:  500,
			ExpectedHeight: 1000,
		},

		{
			Label:          "height and width constrained (portrait) portrait",
			SizeSpec:       height_and_width_constrained_portrait,
			Rect:           portrait,
			ExpectedWidth:  500,
			ExpectedHeight: 1000,
		},

		{
			Label:          "height and width constrained (portrait) landscape",
			SizeSpec:       height_and_width_constrained_portrait,
			Rect:           landscape,
			ExpectedWidth:  250,
			ExpectedHeight: 500,
		},
	}

	for i := range cases {
		c := cases[i]
		r := c.SizeSpec.ToRect(c.Rect)
		if r.Dx() != c.ExpectedWidth {
			t.Error(c.Label, "-- bad width", r.Dx(), "expected", c.ExpectedWidth)
		}
		if r.Dy() != c.ExpectedHeight {
			t.Error(c.Label, "-- bad height", r.Dy(), "expected", c.ExpectedHeight)
		}
	}

}
