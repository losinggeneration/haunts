package mrgnet

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"time"
)

type NetId int64
type GameKey string

const Host_url = "http://mobrulesgames.appspot.com/"

// const Host_url = "http://localhost:8080"

func DoAction(name string, input, output interface{}) error {
	zipit := true
	buf := bytes.NewBuffer(nil)
	var gzw io.Writer
	if zipit {
		gzw = gzip.NewWriter(buf)
	} else {
		gzw = buf
	}
	err := gob.NewEncoder(gzw).Encode(input)
	if err != nil {
		return err
	}
	if zipit {
		gzw.(*gzip.Writer).Close()
	}
	host_url := fmt.Sprintf("%s/%s", Host_url, name)
	// fmt.Printf("Sending %d bytes\n", buf.Len())
	r, err := http.PostForm(host_url, url.Values{"data": []string{string(buf.Bytes())}})
	if err != nil {
		return err
	}

	var gzr io.Reader
	if zipit {
		gzr, err = gzip.NewReader(r.Body)
		if err != nil {
			panic(err.Error())
			return nil
		}
	} else {
		gzr = r.Body
	}
	data, err := ioutil.ReadAll(gzr)
	if err != nil {
		panic(err.Error())
		return nil
	}
	dec := gob.NewDecoder(bytes.NewBuffer(data))
	return dec.Decode(output)
}

// Creates a random id that will be unique among all other engines with high
// probability.
func RandomId() NetId {
	b := big.NewInt(1 << 62)
	v, err := rand.Int(rand.Reader, b)
	if err != nil {
		// uh-oh
		panic(err)
	}
	return NetId(v.Int64())
}

type User struct {
	Id   NetId
	Name string
}

type UpdateUserRequest User
type UpdateUserResponse struct {
	User
	Err string
}

type NewGameRequest struct {
	Id NetId
}

type NewGameResponse struct {
	Err      string
	Name     string
	Game_key GameKey
}

type ListGamesRequest struct {
	Id        NetId
	Unstarted bool
}

type ListGamesResponse struct {
	Err       string
	Games     []Game
	Game_keys []GameKey
}

// Updates an active game by appending a Playback, or updating the last
// playback, with either new State or new Execs
type UpdateGameRequest struct {
	Id        NetId
	Game_key  GameKey
	Round     int
	Intruders bool

	// Exactly one of the following should be set
	Before []byte
	Execs  []byte
	After  []byte
	Script []byte
}

type UpdateGameResponse struct {
	Err string
}

type JoinGameRequest struct {
	Id       NetId
	Game_key GameKey
}

type JoinGameResponse struct {
	Err        string
	Successful bool
}

type StatusRequest struct {
	Id         NetId
	Game_key   GameKey
	Sizes_only bool
}

type StatusResponse struct {
	Err  string
	Game *Game
}

type KillRequest struct {
	Id       NetId
	Game_key GameKey
}

type KillResponse struct {
	Err string
}

type Game struct {
	Name string

	Created time.Time

	Denizens_name  string
	Denizens_id    NetId
	Intruders_name string
	Intruders_id   NetId

	// When in the datastore each of these []byte is a blobstore key for the
	// actual data.  When sent to a user the data is fetched and filled out.
	Before [][]byte
	Execs  [][]byte
	After  [][]byte
	Script []byte

	// If this is non-zero then the game is over and the winner is the player
	// whose NetId matches this value
	Winner NetId
}
