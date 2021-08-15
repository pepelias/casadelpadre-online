package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/olahol/melody"
)

type Stream struct {
	Online  bool                     `json:"online"`
	Streams map[string]string        `json:"streams"`
	Chat    []map[string]interface{} `json:"chat"`
	Viewers int                      `json:"viewers"`
}

var (
	WS     = melody.New()
	STREAM = &Stream{
		Online: true,
		Streams: map[string]string{
			"240p": "http://192.168.1.135:8080/video/video240.mp4",
			"480p": "http://192.168.1.135:8080/video/video480.mp4",
			"720p": "http://192.168.1.135:8080/video/video720.mp4",
		},
		Chat: make([]map[string]interface{}, 0),
	}
)

func main() {
	e := echo.New()
	e.Pre(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "http://192.168.1.135:3000"},
	}))
	Router(e)
	// Websocket
	WS.HandleMessage(func(sess *melody.Session, msg []byte) {
		go func() {
			message := make(map[string]interface{})
			err := json.Unmarshal(msg, &message)
			if err != nil {
				log.Println(err)
				return
			}
			if message["action"] == nil || message["action"].(string) != "chat-message" {
				return
			}
			STREAM.Chat = append(STREAM.Chat, message["data"].(map[string]interface{}))
		}()
		WS.Broadcast(msg)
	})
	WS.HandleConnect(func(s *melody.Session) {
		STREAM.Viewers += 1
		go func() {
			message := map[string]interface{}{
				"action": "viewers-refresh",
				"data": map[string]interface{}{
					"viewers": STREAM.Viewers,
				},
			}
			send(message)
			time.Sleep(time.Second * 2)
			sendTo(s, message)
		}()
	})
	WS.HandleDisconnect(func(s *melody.Session) {
		STREAM.Viewers -= 1
		send(map[string]interface{}{
			"action": "viewers-refresh",
			"data": map[string]interface{}{
				"viewers": STREAM.Viewers,
			},
		})
	})
	e.Start(":8080")
}
func Router(e *echo.Echo) {
	e.GET("/v1/streaming/online", func(c echo.Context) error {
		if STREAM.Online {
			return c.JSON(http.StatusOK, struct {
				Streams map[string]string `json:"streams"`
			}{STREAM.Streams})
		}
		return c.NoContent(http.StatusNotFound)
	})
	e.GET("/v1/streaming", func(c echo.Context) error {
		return c.JSON(http.StatusOK, STREAM)
	})
	e.GET("/v1/websocket", func(c echo.Context) error {
		WS.HandleRequest(c.Response().Writer, c.Request())
		return nil
	})
	// Endpoints para NGIX
	e.Static("/video", "./video")
}
func send(msg map[string]interface{}) {
	message, err := json.Marshal(msg)
	if err != nil {
		return
	}
	WS.Broadcast(message)
}
func sendTo(sess *melody.Session, msg map[string]interface{}) {
	message, err := json.Marshal(msg)
	if err != nil {
		return
	}
	sess.Write(message)
}
