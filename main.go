package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/olahol/melody"
)

type Stream struct {
	Online  bool              `json:"online"`
	Streams map[string]string `json:"streams"`
	Chat    []*Message        `json:"chat"`
	Viewers int               `json:"viewers"`
}

type Message = struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

var (
	WS     = melody.New()
	STREAM = &Stream{
		Streams: make(map[string]string),
		Chat:    make([]*Message, 0),
	}
)

func main() {
	e := echo.New()
	e.Pre(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000"},
	}))
	Router(e)
	// Websocket
	WS.HandleMessage(func(sess *melody.Session, msg []byte) {
		go func() {
			message := &Message{}
			err := json.Unmarshal(msg, message)
			if err != nil {
				log.Println(err)
				return
			}
			STREAM.Chat = append(STREAM.Chat, message)
		}()
		WS.Broadcast(msg)
	})

	e.Start(":8080")
}
func Router(e *echo.Echo) {
	e.GET("/v1/streaming", func(c echo.Context) error {
		return c.JSON(http.StatusOK, STREAM)
	})
	e.GET("/v1/websocket", func(c echo.Context) error {
		WS.HandleRequest(c.Response().Writer, c.Request())
		return nil
	})
	// Endpoints para NGIX
}
