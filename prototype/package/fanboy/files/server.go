package main

import (
	ef "github.com/labstack/echo"
	"fmt"
	"strconv"
	"net/http"
	ctx "context"
	"github.com/labstack/echo/middleware"
	"golang.org/x/net/websocket"
	"encoding/json"
	"github.com/labstack/gommon/log"
	"container/list"
	"io"
)

func prepareServer(port int, staticPath string, communicator *communicator) *server {
	echo := ef.New()
	server := &server{
		echo: echo,
		port: port,
	}

	echo.Use(middleware.Recover())

	echo.Static("/", staticPath)

	echo.GET("/fans", func(context ef.Context) error {
		fans := communicator.fans()
		return context.JSON(http.StatusOK, fans)
	})

	echo.GET("/fans/:id", func(context ef.Context) error {
		fans := communicator.fans()
		sid := context.Param("id")

		id, err := strconv.Atoi(sid)
		if err != nil {
			return context.String(http.StatusBadRequest, "id is not an integer")
		}

		if id < 0 {
			return context.String(http.StatusBadRequest, "id must not be smaller than 0")
		}

		if id > 5 {
			return context.String(http.StatusBadRequest, "id must not be larger than 5")
		}

		return context.JSON(http.StatusOK, fans[id])
	})

	echo.POST("/fans", func(context ef.Context) error {
		sspeed := context.QueryParam("speed")
		speed, err := strconv.Atoi(sspeed)
		if err != nil {
			return context.String(http.StatusBadRequest, "speed is not an integer")
		}

		if speed < 0 {
			return context.String(http.StatusBadRequest, "speed must not be smaller than 0")
		}

		if speed > 100 {
			return context.String(http.StatusBadRequest, "speed must not be larger than 100")
		}

		communicator.setSpeed(-1, speed)
		return context.String(http.StatusOK, "success")
	})

	echo.POST("/fans/:id", func(context ef.Context) error {
		sid := context.Param("id")

		id, err := strconv.Atoi(sid)
		if err != nil {
			return context.String(http.StatusBadRequest, "id is not an integer")
		}

		if id < 0 {
			return context.String(http.StatusBadRequest, "id must not be smaller than 0")
		}

		if id > 5 {
			return context.String(http.StatusBadRequest, "id must not be larger than 5")
		}

		sspeed := context.QueryParam("speed")
		speed, err := strconv.Atoi(sspeed)
		if err != nil {
			return context.String(http.StatusBadRequest, "speed is not an integer")
		}

		if speed < 0 {
			speed = 0
		}

		if speed > 100 {
			speed = 100
		}

		communicator.setSpeed(id, speed)
		return context.String(http.StatusOK, "success")
	})

	echo.GET("/stop", func(context ef.Context) error {
		server.quit <- "fin"
		return context.String(http.StatusOK, "Going down master :)")
	})

	echo.GET("/ws", func(context ef.Context) error {
		websocket.Handler(func(connection *websocket.Conn) {
			var registration *list.Element
			notifier := func(fans []*Fan) {
				val, err := json.Marshal(fans)
				if err != nil {
					log.Warn(err)
				}
				err = websocket.Message.Send(connection, string(val))
				if err != nil {
					log.Warn(err)
					if registration != nil {
						communicator.removeListener(registration)
					}
					connection.Close()
				}
			}
			registration = communicator.addListener(notifier)

			for {
				var msg string
				err := websocket.Message.Receive(connection, &msg)
				if err != nil && err != io.EOF {
					log.Warn(err)
					if registration != nil {
						communicator.removeListener(registration)
					}
					connection.Close()
					break
				}
				if msg != "" {
					var data map[string]interface{}
					json.Unmarshal([]byte(msg), &data)
					fmt.Println(msg)
					fmt.Println(data)
					if data["cmd"] == "setspeed" {
						sspeed := data["speed"].(string)
						speed, err := strconv.Atoi(sspeed)
						if err != nil {
							speed = 45
						}

						if speed < 0 {
							speed = 45
						}

						if speed > 100 {
							speed = 100
						}

						communicator.setSpeed(-1, speed)
					}
				}
			}
		}).ServeHTTP(context.Response(), context.Request())
		return nil
	})

	return server
}

type server struct {
	echo *ef.Echo
	port int
	quit chan string
}

func (s *server) start(quit chan string) {
	s.quit = quit
	err := s.echo.Start(fmt.Sprintf(":%d", s.port))
	if err != nil {
		panic(err)
	}
}

func (s *server) stop(context ctx.Context) {
	s.echo.Shutdown(context)
}
