package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/opentracing/opentracing-go"
)

func startUpRest() {
	addr := fmt.Sprintf(":%d", *port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/home", homeHandler)
	mux.HandleFunc("/async", serviceHandler)
	mux.HandleFunc("/service", serviceHandler)
	mux.HandleFunc("/db", dbHandler)
	fmt.Printf("Go to http://localhost:%d/home to start a request!\n", *port)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func indexHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte(`<a href="/home"> Click here to start a request </a>`))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("Request started"))
	sp := opentracing.StartSpan("GET /home") // Start a new root span.
	defer sp.Finish()

	asyncReq, _ := http.NewRequest("GET", "http://localhost:8080/async", nil)
	// Inject the trace information into the HTTP Headers.
	err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.HTTPHeadersCarrier(asyncReq.Header))
	if err != nil {
		log.Fatalf("%s: Couldn't inject headers (%v)", r.URL.Path, err)
	}

	go func() {
		sleepMilli(50)
		if _, err := http.DefaultClient.Do(asyncReq); err != nil {
			log.Printf("%s: Async call failed (%v)", r.URL.Path, err)
		}
	}()

	sleepMilli(10)
	syncReq, _ := http.NewRequest("GET", "http://localhost:8080/service", nil)
	// Inject the trace info into the headers.
	err = sp.Tracer().Inject(sp.Context(),
		opentracing.TextMap,
		opentracing.HTTPHeadersCarrier(syncReq.Header))
	if err != nil {
		log.Fatalf("%s: Couldn't inject headers (%v)", r.URL.Path, err)
	}
	if _, err = http.DefaultClient.Do(syncReq); err != nil {
		log.Printf("%s: Synchronous call failed (%v)", r.URL.Path, err)
		return
	}
	_, _ = w.Write([]byte("... done!"))
}

func serviceHandler(w http.ResponseWriter, r *http.Request) {
	opName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	var sp opentracing.Span
	spCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap,
		opentracing.HTTPHeadersCarrier(r.Header))
	if err == nil {
		sp = opentracing.StartSpan(opName, opentracing.ChildOf(spCtx))
	} else {
		sp = opentracing.StartSpan(opName)
	}
	defer sp.Finish()

	sleepMilli(50)

	dbReq, _ := http.NewRequest("GET", "http://localhost:8080/db", nil)
	err = sp.Tracer().Inject(sp.Context(),
		opentracing.TextMap,
		opentracing.HTTPHeadersCarrier(dbReq.Header))
	if err != nil {
		log.Fatalf("%s: Couldn't inject headers (%v)", r.URL.Path, err)
	}

	if _, err := http.DefaultClient.Do(dbReq); err != nil {
		sp.LogKV("db request error", err)
	}
}

func dbHandler(w http.ResponseWriter, r *http.Request) {
	var sp opentracing.Span

	spanCtx, err := opentracing.GlobalTracer().Extract(opentracing.TextMap,
		opentracing.HTTPHeadersCarrier(r.Header))
	if err != nil {
		log.Printf("%s: Could not join trace (%v)", r.URL.Path, err)
		return
	}
	if err == nil {
		sp = opentracing.StartSpan("GET /db", opentracing.ChildOf(spanCtx))
	} else {
		sp = opentracing.StartSpan("GET /db")
	}
	defer sp.Finish()
	sleepMilli(25)
}
