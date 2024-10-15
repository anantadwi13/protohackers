package main

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"

	"github.com/anantadwi13/protohackers/8-insecure-sockets-layer/isl"
	"github.com/anantadwi13/protohackers/8-insecure-sockets-layer/util"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	server, err := isl.NewServer(":19090", func(ctx context.Context, w io.Writer, r io.Reader) {
		log.Println("handler begin")

		toys, err := decodeToys(r)
		if err != nil {
			log.Println(err)
			return
		}

		if len(toys) <= 0 {
			log.Println("no toys found")
			return
		}

		sort.Slice(toys, func(i, j int) bool {
			return toys[i].Count > toys[j].Count
		})

		err = encodeToy(w, toys[0])
		if err != nil {
			log.Println(err)
			return
		}

		log.Printf("handler success. %v", len(toys))
	})
	if err != nil {
		log.Fatalln(err)
	}
	go func() {
		log.Println("listening")
		err = server.Listen()
		if err != nil {
			log.Fatalln(err)
		}
	}()

	<-ctx.Done()
	err = server.Shutdown(context.TODO())
	if err != nil {
		log.Fatalln(err)
	}
}

type Toy struct {
	Count int32
	Name  string
}

func decodeToys(r io.Reader) ([]Toy, error) {
	if r == nil {
		return nil, errors.New("nil reader")
	}

	var (
		toys   []Toy
		isEOF  bool
		reader = util.GetBufferReader(isl.MaxLineLength)
	)
	defer util.PutBufferReader(reader)
	reader.Reset(r)

	for !isEOF {
		slice, err := reader.ReadSlice('x')
		if err != nil {
			return nil, err
		}
		slice = slice[:len(slice)-1]
		count, err := strconv.ParseInt(string(slice), 10, 32)
		if err != nil {
			return nil, err
		}
		slice, err = reader.ReadSlice(',')
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			isEOF = true
		}
		if !isEOF {
			slice = slice[:len(slice)-1]
		}
		toys = append(toys, Toy{
			Count: int32(count),
			Name:  string(slice),
		})
	}
	return toys, nil
}

func encodeToy(w io.Writer, toy Toy) error {
	count := strconv.FormatInt(int64(toy.Count), 10)
	n, err := w.Write([]byte(count))
	if err != nil {
		return err
	}
	if n != len(count) {
		return errors.New("encode error. data written is not equal")
	}
	n, err = w.Write([]byte("x"))
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("encode error. data written is not equal")
	}
	n, err = w.Write([]byte(toy.Name))
	if err != nil {
		return err
	}
	if n != len(toy.Name) {
		return errors.New("encode error. data written is not equal")
	}
	return nil
}
