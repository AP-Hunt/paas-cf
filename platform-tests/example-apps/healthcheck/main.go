package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/beevik/ntp"
)

func main() {
	addr := ":" + os.Getenv("PORT")
	fmt.Println("Listening on", addr)
	http.HandleFunc("/", staticHandler)
	http.HandleFunc("/db", dbHandler)
	http.HandleFunc("/mongo-test", mongoHandler)
	http.HandleFunc("/elasticsearch-test", elasticsearchHandler)
	http.HandleFunc("/redis-test", redisHandler)
	go func() {
		for {
			response, err := ntp.Query("0.pool.ntp.org")
			if err != nil {
				fmt.Println("Time ntp error:", err)
			} else {
				fmt.Println(
					"response.offset:", response.ClockOffset,
					"response.time:", response.Time,
					"response.RRT:", response.RTT,
					"time.now():", time.Now(),
				)
			}
			time.Sleep(1 * time.Second)
		}
	}()
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=0,no-store,no-cache")
	http.ServeFile(w, r, "static/"+r.URL.Path[1:])
}

func writeJson(w http.ResponseWriter, data interface{}) {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "max-age=0,no-store,no-cache")
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func buildTLSConfigWithCACert(caCertBase64 string) (*tls.Config, error) {
	ca, err := base64.StdEncoding.DecodeString(caCertBase64)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(ca)
	if !ok {
		return nil, fmt.Errorf("Failed to parse CA certificate")
	}

	return &tls.Config{RootCAs: roots}, nil
}
