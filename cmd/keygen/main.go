package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"regexp"
	"runtime"
	"time"
)

func main() {
	pubkeyRe := regexp.MustCompile(`83e(0[1-9]|1[0-2])23$`)
	nGoroutines := runtime.NumCPU()

	vanityReStr := flag.String("vanity", "",
		"A vanity regex that the public key must also match.")
	flag.Parse()

	var vanityRe *regexp.Regexp
	if *vanityReStr != "" {
		vanityRe = regexp.MustCompile(*vanityReStr)
	}

	valid := func(pubkey []byte) bool {
		// Fail fast if the key doesn't match '83e', so the more expensive
		// check on the hex string happens on fewer candidates.
		suffix := pubkey[len(pubkey)-4:]
		if binary.BigEndian.Uint16(suffix)&0xfff != 0x83e {
			return false
		}

		pubhex := hex.EncodeToString(pubkey)

		if vanityRe != nil && !vanityRe.MatchString(pubhex) {
			return false
		}

		return pubkeyRe.MatchString(pubhex)
	}

	start := time.Now()
	counts := make([]uint64, nGoroutines)

	keys := make(chan []byte)

	for i := 0; i < runtime.NumCPU(); i++ {
		go func(index int) {
			priv, err := generateMatch(rand.Reader, valid)
			if err != nil {
				log.Printf("generateMatch: %s", err)
				return
			}

			counts[index]++

			select {
			case keys <- priv:
				close(keys)
			default:
			}
		}(i)
	}

	key := <-keys

	pub := key[len(key)-ed25519.PublicKeySize:]

	filename := fmt.Sprintf("spring-83-keypair-%s-%x.txt",
		time.Now().Format("2006-01-02"), pub[:6])

	content := fmt.Sprintf("%x\n", key)

	ioutil.WriteFile(filename, []byte(content), 0644)

	fmt.Printf("Checked %d candidates in %s\n", sum(counts), time.Since(start).Truncate(time.Millisecond))
	fmt.Printf("Wrote: %s\n", filename)
}

func generateMatch(r io.Reader, valid func([]byte) bool) ([]byte, error) {
	for {
		pub, priv, err := ed25519.GenerateKey(r)
		if err != nil {
			return nil, err
		}

		if valid(pub) {
			return priv, nil
		}
	}
}

func sum(vv []uint64) uint64 {
	var sum uint64
	for _, v := range vv {
		sum += v
	}
	return sum
}
