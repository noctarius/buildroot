package main

import (
	ef "github.com/labstack/echo"
	"fmt"
	"strconv"
	"net/http"
)

func prepareServer(port int, communicator *communicator) *server {
	echo := ef.New()
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

	return &server{
		echo:         echo,
		port:         port,
	}
}

type server struct {
	echo         *ef.Echo
	port         int
}

func (s *server) start() {
	err := s.echo.Start(fmt.Sprintf(":%d", s.port))
	if err != nil {
		panic(err)
	}
}
