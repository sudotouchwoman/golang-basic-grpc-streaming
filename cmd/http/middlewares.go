package main

import (
	"log"
	"net/http"
	"time"
)

// Basic middleware (guess it is not that useful when
// it comes to a websocket connection, yet whatever)
type proxyResponseWriter struct {
	// helper struct to extract http response code
	// once handler returns
	http.ResponseWriter
	code int
}

func (rwi *proxyResponseWriter) WriteHeader(code int) {
	rwi.code = code
	rwi.ResponseWriter.WriteHeader(code)
}

func LogRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Default().Printf("%s %v (%s)", r.Method, r.URL, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func LogResponseCode(next http.Handler) http.Handler {
	// log incoming requests and their corresponding response codes
	// the response code is 200 by default as custom handlers may not
	// call the WriteHeader method directly
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyWriter := &proxyResponseWriter{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(proxyWriter, r)
		log.Printf("%s %v - %d (%s)", r.Method, r.URL, proxyWriter.code, time.Since(start))
	})
}

func PanicRecovery(next http.Handler) http.Handler {
	// recover if for some reason execution panics
	// and respond with a 500 status code
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Default().Printf("Panicked: %+v", err)
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
