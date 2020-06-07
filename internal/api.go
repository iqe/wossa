package wossamessa

import (
	"bytes"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

// Run runs the web server and blocks forever
func Run() {
	r := gin.New()
	r.Use(gin.Recovery())

	// Static file hosting for web ui
	r.Static("/public", "./public")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/public/index.html")
	})

	// API
	r.GET("/api/v1/meter.json", func(c *gin.Context) {
		m, err := loadMeter()
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, m)
	})
	r.POST("/api/v1/meter.json", func(c *gin.Context) {
		m, err := loadMeter()
		if err != nil {
			c.AbortWithError(500, fmt.Errorf("Failed to load meter: %s", err))
			return
		}

		err = c.BindJSON(&m)
		if err != nil {
			c.AbortWithError(400, err)
			return
		}

		m, err = UpdateMeter(m)
		if err != nil {
			c.AbortWithError(500, fmt.Errorf("failed to update meter: %s", err))
			return
		}
		c.JSON(200, m)
	})

	r.GET("/api/v1/config.json", func(c *gin.Context) {
		config, err := loadConfig()
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, config)
	})
	r.POST("/api/v1/config.json", func(c *gin.Context) {
		config, err := loadConfig()
		if err != nil {
			c.AbortWithError(500, err)
			return
		}

		err = c.BindJSON(&config)
		if err != nil {
			log.Printf("Failed to bindjson: %s\n", err)
			c.AbortWithError(400, err)
			return
		}
		saveConfig(config)
		c.JSON(200, config)
	})

	r.GET("/api/v1/preview.jpg", func(c *gin.Context) {
		jpeg, err := loadPreview()
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		reader := bytes.NewReader(jpeg)

		c.DataFromReader(200, int64(len(jpeg)), "image/jpeg", reader, map[string]string{})
	})

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
