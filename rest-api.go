package main

import (
	"context"
	"net/http"
	"time"

	"github.com/Devisree146/go-cache-api/cache" // Update with your GitHub username and repo name
	"github.com/gin-gonic/gin"
)

var (
	inMemoryCache = cache.NewInMemoryCache()
	redisCache    = cache.NewRedisCache("localhost:6379", "", 0)
)

func main() {
	r := gin.Default()

	r.POST("/cache", handleCachePost)
	r.GET("/cache/:key", handleCacheGet)
	r.DELETE("/cache/:key", handleCacheDelete)

	r.Run(":8080")
}

func handleCachePost(c *gin.Context) {
	var entry cache.CacheEntry
	if err := c.BindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode JSON"})
		return
	}

	ctx := context.Background()
	// You can switch between inMemoryCache and redisCache
	if err := inMemoryCache.Set(ctx, entry.Key, entry.Value, time.Duration(entry.TTL)*time.Second); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Key set successfully", "key": entry.Key})
}

func handleCacheGet(c *gin.Context) {
	key := c.Param("key")
	ctx := context.Background()
	value, err := inMemoryCache.Get(ctx, key) // You can switch between inMemoryCache and redisCache
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cache"})
		return
	}
	if value == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"key": key, "value": value})
}

func handleCacheDelete(c *gin.Context) {
	key := c.Param("key")
	ctx := context.Background()
	if err := inMemoryCache.Delete(ctx, key); err != nil { // You can switch between inMemoryCache and redisCache
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Key deleted successfully", "key": key})
}
