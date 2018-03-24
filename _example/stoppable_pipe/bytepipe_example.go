package main

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/kithix/thing"
)

var nmeaSentence = []byte(`$GPRMC,183729,A,3907.356,N,12102.482,W,000.0,360.0,080301,015.5,E*6F
$GPRMC,183729,A,3907.356,N,12102.482,W,000.0,360.0,080301,015.5,E*6F
$GPGSA,A,3,02,,,07,,09,24,26,,,,,1.6,1.6,1.0*3D
$GPGSV,2,1,08,02,43,088,38,04,42,145,00,05,11,291,00,07,60,043,35*71
$GPGLL,3907.360,N,12102.481,W,183730,A*33
`)

func writeBytes(writer io.Writer) error {
	_, err := writer.Write(nmeaSentence)
	time.Sleep(500 * time.Millisecond)
	return err
}

func readBytes(reader io.Reader) error {
	_, err := io.Copy(os.Stdout, reader)
	return err
}

func main() {
	r, w := io.Pipe()
	writerPipe := thing.MakeStoppable(thing.BuildStoppableFunc(
		thing.DoesNothing,
		func() error {
			err := writeBytes(w)
			if err != nil {
				w.CloseWithError(err)
				return err
			}
			return err
		}, func() error {
			return w.Close()
		}))

	readerPipe := thing.MakeStoppable(thing.BuildStoppableFunc(
		thing.DoesNothing,
		func() error {
			err := readBytes(r)
			if err != nil {
				defer r.CloseWithError(err)
			}
			return err
		}, func() error {
			return r.Close()
		}))

	time.Sleep(2 * time.Second)
	log.Println("Stopping writer")
	err := writerPipe.Stop()
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Writer successfully stopped")
	}
	time.Sleep(1 * time.Second)
	log.Println("Stopping reader")
	err = readerPipe.Stop()
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Reader successfully stopped")
	}
}
