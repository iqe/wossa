package wossamessa

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

// Run runs the web server and blocks forever
func Run() {
	r := gin.Default()

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
			c.AbortWithError(500, err)
			return
		}

		err = c.BindJSON(&m)
		if err != nil {
			c.AbortWithError(400, err)
			return
		}

		saveMeter(m)
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
