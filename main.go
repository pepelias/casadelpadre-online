package main

import (
	"casadelpadre-online/config"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/olahol/melody"
)

var conf = config.Get()

type Stream struct {
	Online  bool                     `json:"online"`
	Streams map[string]string        `json:"streams"`
	Chat    []map[string]interface{} `json:"chat"`
	Viewers int                      `json:"viewers"`
}

var (
	WS     = melody.New()
	STREAM = &Stream{
		Online: false,
		Streams: map[string]string{
			"240p": conf.Qualities.Low,
			"480p": conf.Qualities.Mid,
			"720p": conf.Qualities.High,
		},
		Chat: make([]map[string]interface{}, 0),
	}
	Online = &struct {
		High bool
		Mid  bool
		Low  bool
	}{}
)

func main() {
	e := echo.New()
	e.Pre(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: conf.Server.Cors,
		AllowMethods: []string{"POST", "GET"},
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

	if conf.SSL.Cert != "" && conf.SSL.Key != "" {
		go func() {
			e.StartTLS(conf.Server.SecurePort, conf.SSL.Cert, conf.SSL.Key)
		}()
	}

	e.Start(conf.Server.Port)
}
func Router(e *echo.Echo) {
	e.GET("/v1/streaming/online", func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			Online  bool              `json:"online"`
			Streams map[string]string `json:"streams"`
		}{STREAM.Online, STREAM.Streams})
	})
	e.GET("/v1/streaming", func(c echo.Context) error {
		return c.JSON(http.StatusOK, STREAM)
	})
	e.GET("/v1/websocket", func(c echo.Context) error {
		WS.HandleRequest(c.Response().Writer, c.Request())
		return nil
	})
	// Endpoints para NGIX
	e.POST("/v1/streaming/on", func(c echo.Context) error {
		body := struct {
			Name string `form:"name"`
		}{}
		err := c.Bind(&body)
		if err != nil {
			log.Println(err)
			return c.NoContent(http.StatusBadRequest)
		}

		switch body.Name {
		case conf.Streams.High:
			Online.High = true
		case conf.Streams.Mid:
			Online.Mid = true
		case conf.Streams.Low:
			Online.Low = true
		}

		if Online.High && Online.Mid && Online.Low {
			// Están los 3 streams
			STREAM.Online = true
			message := map[string]interface{}{
				"action": "stream-started",
				"data": map[string]interface{}{
					"viewers": STREAM.Viewers,
				},
			}
			send(message)
		}
		return c.NoContent(http.StatusOK)
	})
	e.POST("/v1/streaming/off", func(c echo.Context) error {
		STREAM.Online = false
		message := map[string]interface{}{
			"action": "stream-ended",
			"data": map[string]interface{}{
				"viewers": STREAM.Viewers,
			},
		}
		send(message)
		STREAM.Chat = make([]map[string]interface{}, 0)
		// Reset
		Online.High = false
		Online.Mid = false
		Online.Low = false
		return c.NoContent(http.StatusOK)
	})
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
