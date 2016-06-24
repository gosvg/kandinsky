package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/gosvg/kandinsky"
)

func main() {
	var port string

	flag.StringVar(&port, "http", ":6000", "http port to listen on")

	flag.Parse()

	http.Handle("/viz", http.HandlerFunc(valHandler))
	http.Handle("/struct", http.HandlerFunc(testing))
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
		marshalTo(w, i)
	case "float":
		f, err1 := strconv.ParseFloat(req.FormValue("v"), 64)
		if err1 != nil {
			w.WriteHeader(400)
			return
		}
		marshalTo(w, f)
	case "bool":
		b, err1 := strconv.ParseBool(req.FormValue("v"))
		if err1 != nil {
			w.WriteHeader(400)
			return
		}
		marshalTo(w, b)
	default:
		w.WriteHeader(400)
		return
	}
}

func marshalTo(w http.ResponseWriter, i interface{}) {
	b, errMarshal := kandinsky.Marshal(i, 96)
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

func testing(w http.ResponseWriter, req *http.Request) {
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

	marshalTo(w, s)
}
