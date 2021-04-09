package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opentracing/opentracing-go"
)

func startUpWebsocket() {
	go func() {
		time.Sleep(5 * time.Second)

		// inject span
		newHeader := http.Header{}
		sp := opentracing.StartSpan(fmt.Sprintf("websocket client: %d", time.Now().UnixNano())) // Start a new root span.
		defer sp.Finish()
		sp.SetTag("key", "value")
		err := sp.Tracer().Inject(sp.Context(), opentracing.TextMap, opentracing.HTTPHeadersCarrier(newHeader))
		if err != nil {
			log.Fatalf("couldn't inject headers (%+v)", err)
		}

		////
		u := url.URL{
			Scheme: "ws",
			Host:   "localhost:8899",
			Path:   "/echo",
		}
		log.Printf("connecting to %s\n", u.String())

		c, _, err := websocket.DefaultDialer.Dial(u.String(), newHeader)
		if err != nil {
			log.Fatal("dial:", err)
		}
		defer c.Close()

		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Printf("read err: %+v\n", err)
					return
				}
				log.Printf("recv: %s\n", message)
			}
		}()

		ticker := time.NewTicker(200 * time.Millisecond)
		timer := time.NewTimer(500 * time.Millisecond)

		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
				if err != nil {
					log.Printf("write err: %+v\n", err)
					return
				}
				log.Printf("writed: %s\n", t.String())
			case <-timer.C:
				log.Printf("timer fired\n")

				err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Printf("write close err: %+v\n", err)
					return
				}
				select {
				case <-done:
				case <-time.After(time.Second):
				}
				return
			}
		}
		////
	}()

	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		upgrader := &websocket.Upgrader{
			// skip cross domain check
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		defer func() {
			log.Println("disconnect !!")
			c.Close()
		}()
		for {
			mtype, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			log.Printf("receive: %s\n", msg)

			// extract span
			var sp opentracing.Span
			spCtx, err := opentracing.GlobalTracer().Extract(
				opentracing.TextMap,
				opentracing.HTTPHeadersCarrier(r.Header),
			)
			if err == nil {
				sp = opentracing.StartSpan(fmt.Sprintf("received: %d", time.Now().UnixNano()), opentracing.ChildOf(spCtx))
			} else {
				sp = opentracing.StartSpan(fmt.Sprintf("received: %d", time.Now().UnixNano()))
			}
			defer sp.Finish()

			err = c.WriteMessage(mtype, msg)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	})
	log.Println("server start at :8899")
	log.Fatal(http.ListenAndServe(":8899", nil))
}
