package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	ping "github.com/digineo/go-ping"
	"github.com/kithix/thing"
)

func main() {
	var p *ping.Pinger
	var restartable thing.Restartable
	var target = "127.0.0.1"
	iter := 0
	restartable = thing.NewWatchdog(
		thing.MakeRestartable(
			thing.BuildStoppableFunc(
				func() error {
					var err error
					log.Println("Creating pinger for " + target)
					p, err = ping.New("0.0.0.0", "")
					if err != nil {
						p = nil
						return err
					}
					log.Println("Created pinger for " + target)
					return nil
				},
				func() error {
					iter++
					if iter%3 == 0 {
						return errors.New("We are failing on purpose")
					}
					remote, err := net.ResolveIPAddr("ip4", target)
					if err != nil {
						return err
					}
					rtt, err := p.PingRTT(remote)
					if err != nil {
						return err
					}
					fmt.Printf("ping %s (%s) rtt=%v\n", target, remote, rtt)
					time.Sleep(1 * time.Second)
					return nil
				},
				func() error {
					log.Println("Closing pinger for " + target)
					p.Close()
					log.Println("Closed pinger for " + target)
					return nil
				},
			),
		),
		func(err error) {
			if err != nil {
				log.Println(err)
			}
			err = restartable.Restart()
			if err != nil {
				log.Println(err)
			}
		},
	)

	err := restartable.Start()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		err := restartable.Start()
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		w.Write([]byte("Started!"))
	})

	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		err := restartable.Stop()
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		w.Write([]byte("Stopped"))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
