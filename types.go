package kandinsky

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"reflect"
	"sync"

	"github.com/gosvg/gosvg"
)

// Marshal marshals a value into a valid SVG document represented as a byte slice.
// Users should note that size here represents a nominal document size; SVG documents
// are vector graphics and as such do not have a notion of image size. This will simply
// provide a sensible default for web browsers or other viewers when displaying the output.
func Marshal(i interface{}, size float64) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("kandinsky: invalid marshal size")
	}

	var b bytes.Buffer

	s := gosvg.NewSVG(size, size)
	e := &encodeState{SVG: s, size: size}

	err := e.marshal(reflect.ValueOf(i))
	if err != nil {
		return nil, err
	}

	s.Render(&b)

	return b.Bytes(), nil
}

// encodeState encodes data into a bytes.Buffer.
type encodeState struct {
	*gosvg.SVG // accumulated output
	size       float64
}

func (e *encodeState) marshal(v reflect.Value) error {
	f, err := valueEncoder(v)
	if err != nil {
		return err
	}

	return f(e, v)
}

type encoderFunc func(*encodeState, reflect.Value) error

// UnsupportedTypeError represents an error condition where kandinsky does not support
// the provided type for marshaling.
type UnsupportedTypeError struct {
	t reflect.Type
}

func (e UnsupportedTypeError) Error() string {
	return fmt.Sprintf("goviz: unsupported type %s", e.t.String())
}

// InvalidValueError represents an error condition where the given value for a type
// is not marshalable by kandinsky.
type InvalidValueError struct {
	v reflect.Value
}

func (e InvalidValueError) Error() string {
	t := e.v.Type()
	return fmt.Sprintf("goviz: invalid value %s for type %s", e.v.String, t.String())
}

func valueEncoder(v reflect.Value) (encoderFunc, error) {
	if !v.IsValid() {
		return nil, InvalidValueError{v: v}
	}

	return typeEncoder(v.Type())
}

func newTypeEncoder(t reflect.Type) (encoderFunc, error) {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder, nil
	case reflect.Float32, reflect.Float64:
		return floatEncoder, nil
	case reflect.Bool:
		return boolEncoder, nil
	case reflect.Struct:
		return structEncoder, nil
	default:
		return nil, UnsupportedTypeError{t: t}
	}
}

var encoderCache struct {
	m map[reflect.Type]encoderFunc
	sync.RWMutex
}

func typeEncoder(t reflect.Type) (encoderFunc, error) {
	encoderCache.RLock()
	f := encoderCache.m[t]
	encoderCache.RUnlock()

	if f != nil {
		return f, nil
	}

	f, err := newTypeEncoder(t)
	if err != nil {
		return nil, err
	}

	encoderCache.Lock()
	if encoderCache.m == nil {
		encoderCache.m = make(map[reflect.Type]encoderFunc)
	}

	encoderCache.m[t] = f
	encoderCache.Unlock()

	return f, nil
}

func intEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size
	val := int(v.Int())

	color := "black"
	if val <= 0 {
		color = "red"
		val = -val
	}
	cellSz := (sz / 8) - 1

	for y := sz - cellSz - 1; y >= 0; y -= (cellSz + 1) {
		for x := sz - cellSz - 1; x >= 0; x -= (cellSz + 1) {
			bit := val % 2
			if bit == 1 {
				r := e.Rect(x, y, cellSz, cellSz)
				r.Style.Set("stroke-width", "0")
				r.Style.Set("fill", color)
			}
			val = val >> 1
			if val == 0 {
				return nil
			}
		}
	}

	return nil
}

func floatEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size
	val := float64(v.Float())
	val *= float64(sz / 2)

	fill := "black"
	if val <= 0 {
		fill = "red"
		val = -val
	}

	x := sz / 2
	y := x
	c := e.Circle(x, y, val)
	c.Style.Set("fill", fill)
	c.Style.Set("stroke", fill)

	return nil
}

func boolEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size

	val := v.Bool()
	sz = sz - 2

	// y = sin(1/3 * pi)*sz for making equilateral triangle
	sinY := 0.866 * sz
	margin := (sz - sinY) / 2

	x1, x2, x3 := 1.0, sz/2, sz+1
	y1, y2, y3 := sinY, 0.0, sinY

	if val == false {
		y1, y2, y3 = 0, sinY, 0
	}

	pts := []gosvg.Point{{x1, y1}, {x2, y2}, {x3, y3}}
	g := e.Group()
	g.Transform.Translate(0, margin)
	p := g.Polygon(pts...)
	p.Style.Set("stroke-width", "0")
	p.Style.Set("fill", "black")

	return nil
}

func structEncoder(e *encodeState, v reflect.Value) error {
	log.Printf("got struct %#v", v)
	sz := e.size

	t := v.Type()
	numFields := t.NumField()

	// short circuit if numFields is 0
	if numFields == 0 {
		return nil
	}

	width := int(math.Ceil(math.Sqrt(float64(numFields))))
	cellSz := sz / float64(width)

	x := 0
	y := 0
	g := e.Group()
	for i := 0; i < numFields; i++ {
		xF := float64(x) * cellSz
		yF := float64(y) * cellSz
		s := g.SVG(xF, yF, cellSz, cellSz)

		enc := &encodeState{
			SVG:  s,
			size: cellSz,
		}

		errMarshal := enc.marshal(v.Field(i))
		if errMarshal != nil {
			return errMarshal
		}

		x++
		if x == width {
			y++
			x = 0
		}
	}

	return nil
}
