package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gosvg/kandinsky"
)

func main() {
	var port string

	flag.StringVar(&port, "http", ":8080", "http port to listen on")

	flag.Parse()

	http.Handle("/viz", http.HandlerFunc(valHandler))
	http.Handle("/struct", http.HandlerFunc(structs))
	http.Handle("/slice", http.HandlerFunc(slice))

	log.Printf("listening on %s", port)

	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func valHandler(w http.ResponseWriter, req *http.Request) {
	switch req.FormValue("type") {
	case "int":
		i, err1 := strconv.Atoi(req.FormValue("v"))
		if err1 != nil {
			w.WriteHeader(400)
			return
		}
		marshalTo(w, 96, i)
	case "float":
		f, err1 := strconv.ParseFloat(req.FormValue("v"), 64)
		if err1 != nil {
			w.WriteHeader(400)
			return
		}
		marshalTo(w, 96, f)
	case "bool":
		b, err1 := strconv.ParseBool(req.FormValue("v"))
		if err1 != nil {
			w.WriteHeader(400)
			return
		}
		marshalTo(w, 96, b)
	default:
		w.WriteHeader(400)
		return
	}
}

func marshalTo(w http.ResponseWriter, sz float64, i interface{}) {
	b, errMarshal := kandinsky.Marshal(i, sz)
	if errMarshal != nil {
		log.Print(errMarshal)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	_, errWrite := w.Write(b)

	if errWrite != nil {
		log.Print(errWrite)
	}
}

func structs(w http.ResponseWriter, req *http.Request) {
	var s struct {
		X int
		Y float64
		Z bool
		s int
	}

	s.X = -1234
	s.Y = 0.73
	s.Z = true
	s.s = 11235813

	marshalTo(w, 96, s)
}

func slice(w http.ResponseWriter, req *http.Request) {
	type inner struct {
		Bs   []bool
		Strs []string
		F    float64
	}

	type obj struct {
		X int
		F float64
		W string
		I inner
		M map[int][]int
	}

	var s []obj

	for i := 0; i < 16; i++ {
		var iObj inner
		xor := i ^ (i >> 1)
		for j := 0; j < 8; j++ {
			b := (xor & 1) == 0
			iObj.Bs = append(iObj.Bs, b)
			iObj.Strs = append(iObj.Strs, fmt.Sprintf("%v", b))
			xor = xor >> 1
		}
		iObj.F = math.Cos(float64(i) / 2.0)
		m := make(map[int][]int)
		for j := 0; j < i; j++ {
			var js = make([]int, j)
			for k := 0; k < j; k++ {
				js[k] = k
			}
			m[j] = js
		}
		o := obj{
			X: i,
			F: math.Sin(float64(i) / 4.0),
			W: fmt.Sprintf("%d", i),
			I: iObj,
			M: m,
		}
		s = append(s, o)
	}

	marshalTo(w, 900, s)
}
