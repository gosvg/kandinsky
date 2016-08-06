package kandinsky

import (
	"bytes"
	"fmt"
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
	g := s.Group()
	e := &encodeState{Group: g, size: size}

	err := e.marshal(reflect.ValueOf(i))
	if err != nil {
		return nil, err
	}

	s.Render(&b)

	return b.Bytes(), nil
}

// encodeState encodes data into a bytes.Buffer.
type encodeState struct {
	*gosvg.Group // accumulated output
	size         float64
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
	return fmt.Sprintf("goviz: unsupported type %s (kind %s)", e.t.String(), e.t.Kind())
}

// InvalidValueError represents an error condition where the given value for a type
// is not marshalable by kandinsky.
type InvalidValueError struct {
	v reflect.Value
}

func (e InvalidValueError) Error() string {
	return fmt.Sprintf("goviz: invalid value %s", e.v.String())
}

func valueEncoder(v reflect.Value) (encoderFunc, error) {
	if !v.IsValid() {
		panic(InvalidValueError{v: v})
		return nil, InvalidValueError{v: v}
	}

	return typeEncoder(v.Type())
}

func newTypeEncoder(t reflect.Type) (encoderFunc, error) {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder, nil
	case reflect.Uint8:
		return byteEncoder, nil
	case reflect.Float32, reflect.Float64:
		return floatEncoder, nil
	case reflect.Bool:
		return boolEncoder, nil
	case reflect.String:
		return stringEncoder, nil
	case reflect.Struct:
		return structEncoder, nil
	case reflect.Slice:
		return sliceEncoder, nil
	case reflect.Map:
		return mapEncoder, nil
	case reflect.Interface:
		return ifaceEncoder, nil
	case reflect.Ptr:
		return ptrEncoder, nil
	default:
		return nil, UnsupportedTypeError{t: t}
	}
}

var encoderCache struct {
	m map[reflect.Type]encoderFunc
	sync.RWMutex
}

// we try to cache types here because building encoders is expensive.
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

	// lock the cache and initialize it
	encoderCache.Lock()
	if encoderCache.m == nil {
		encoderCache.m = make(map[reflect.Type]encoderFunc)
	}

	encoderCache.m[t] = f
	encoderCache.Unlock()

	return f, nil
}

func ifaceEncoder(e *encodeState, v reflect.Value) error {
	return e.marshal(v.Elem())
}

func ptrEncoder(e *encodeState, v reflect.Value) error {
	elem := v.Elem()
	if !elem.IsValid() {
		return nil
	}
	return e.marshal(elem)
}

// ints in kandinsky encode as 8x8 (64-bit) matrices. MSB
// is at the top left, LSB is in the lower right. positive
// integers are rendered in black and negative in red.
func intEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size
	val := int(v.Int())

	color := "black"
	if val <= 0 {
		color = "red"
		val = -val
	}
	factor := .1
	cellSz := (sz / 8)
	blkSz := cellSz * (1 - factor)
	margin := cellSz * (factor / 2)

	for y := sz - cellSz; y > 0; y -= cellSz {
		for x := sz - cellSz; x > 0; x -= cellSz {
			bit := val % 2
			if bit == 1 {
				r := e.Rect(x+margin, y-margin, blkSz, blkSz)
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

// bytes render as a set of up to 8 stacked bars.
func byteEncoder(e *encodeState, v reflect.Value) error {
	val := v.Uint()
	sz := e.size
	h := sz / 8.0
	factor := .2
	blkH := h * (1 - factor)
	margin := h * (factor / 2)

	for y := sz - h; y > 0; y -= h {
		bit := val % 2
		if bit == 1 {
			r := e.Rect(margin, y-margin, sz-margin, blkH)
			r.Style.Set("stroke-width", "0")
			r.Style.Set("fill", "black")
		}
		val = val >> 1
		if val == 0 {
			return nil
		}
	}

	return nil
}

// strings render as a slice of bytes.
func stringEncoder(e *encodeState, v reflect.Value) error {
	b := reflect.ValueOf([]byte(v.String()))

	return e.marshal(b)
}

// floats currently only render in [-1, 1] as a circle filling a
// proportionate amount of the alloted space.
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

// bools render as a triangle pointing up for true and down for
// false.
func boolEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size
	factor := .25
	factorD2 := factor / 2
	h := sz * (1 - factor)
	color := "black"

	val := v.Bool()

	// y = sin(1/3 * pi)*h for making equilateral triangle
	sinY := 0.866 * h

	x1, x2, x3 := sz*factorD2, sz/2, sz*(1-factorD2)
	hi, lo := x1, sinY+x1

	if val == false {
		hi, lo = lo, hi
		color = "red"
	}

	pts := []gosvg.Point{{x1, lo}, {x2, hi}, {x3, lo}}
	p := e.Polygon(pts...)
	p.Style.Set("stroke-width", "0")
	p.Style.Set("fill", color)

	return nil
}

// structEncoder renders the fields of the struct in order of their
// declaration in source. fields are displayed in a square grid which
// is guaranteed to have enough cells to render all fields.
func structEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size

	t := v.Type()
	numFields := t.NumField()

	// short circuit if numFields is 0
	if numFields == 0 {
		return nil
	}

	width := int(math.Ceil(math.Sqrt(float64(numFields))))
	cellSz := sz / float64(width)
	cellPt := 1 / float64(width)

	x := 0
	y := 0
	g := e.Group.Group()
	for i := 0; i < numFields; i++ {
		xF := float64(x) * cellSz
		yF := float64(y) * cellSz
		s := g.Group()
		s.Transform.Translate(xF, yF)
		s.Transform.Scale(cellPt, cellPt)

		enc := &encodeState{
			Group: s,
			size:  sz,
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

// mapEncoder encodes maps as a slice of key-value pairs.
func mapEncoder(e *encodeState, v reflect.Value) error {
	type kvPair struct {
		k1 interface{}
		v1 interface{}
		k2 interface{}
		v2 interface{}
	}

	keys := v.MapKeys()
	kvPairs := make([]kvPair, 0, (len(keys)+1)/2)

	if len(keys) == 0 {
		return nil
	}

	var (
		p     kvPair
		isOdd bool
	)

	for _, k := range keys {
		val := v.MapIndex(k)
		if !isOdd {
			p.k1, p.v1 = k.Interface(), val.Interface()
		} else {
			p.k2, p.v2 = k.Interface(), val.Interface()
			kvPairs = append(kvPairs, p)
		}

		isOdd = !isOdd
	}

	if !isOdd {
		kvPairs = append(kvPairs, p)
	}

	return sliceEncoder(e, reflect.ValueOf(kvPairs))

}

// sliceEncoder acts similarly to structEncoder, except instead of rendering
// fields in each grid cell the encoder renders indexes.
func sliceEncoder(e *encodeState, v reflect.Value) error {
	sz := e.size

	length := v.Len()

	// short circuit if length is 0
	if length == 0 {
		return nil
	}

	width := int(math.Ceil(math.Sqrt(float64(length))))
	cellSz := sz / float64(width)
	cellPt := 1 / float64(width)

	x := 0
	y := 0
	g := e.Group.Group()
	for i := 0; i < length; i++ {
		xF := float64(x) * cellSz
		yF := float64(y) * cellSz
		s := g.Group()
		s.Transform.Translate(xF, yF)
		s.Transform.Scale(cellPt, cellPt)

		enc := &encodeState{
			Group: s,
			size:  sz,
		}

		errMarshal := enc.marshal(v.Index(i))
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
