package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Webhook struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	CreatedAt time.Time `json:"created_at"`
}

var webhooks sync.Map

type WebhookRequest struct {
	URL    string   `json:"url" binding:"required"`
	Events []string `json:"events" binding:"required"`
}

type WebhookEvent struct {
	Event     string      `json:"event"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func RegisterWebhook(c *gin.Context) {
	var req WebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Events) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one event is required"})
		return
	}

	validEvents := map[string]bool{
		"database.created": true,
		"database.deleted": true,
		"branch.created":  true,
		"branch.deleted":  true,
		"query.error":     true,
		"pool.exhausted":  true,
	}

	for _, event := range req.Events {
		if !validEvents[event] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid event: %s", event)})
			return
		}
	}

	webhook := Webhook{
		ID:        fmt.Sprintf("wh_%d", time.Now().UnixNano()),
		URL:       req.URL,
		Events:    req.Events,
		CreatedAt: time.Now(),
	}

	webhooks.Store(webhook.ID, webhook)

	log.Printf("Webhook registered: %s for events: %v", webhook.URL, webhook.Events)

	c.JSON(http.StatusCreated, webhook)
}

func ListWebhooks(c *gin.Context) {
	var result []Webhook
	webhooks.Range(func(key, value interface{}) bool {
		result = append(result, value.(Webhook))
		return true
	})

	c.JSON(http.StatusOK, result)
}

func DeleteWebhook(c *gin.Context) {
	id := c.Param("id")

	if _, ok := webhooks.LoadAndDelete(id); ok {
		c.JSON(http.StatusOK, gin.H{"message": "webhook deleted"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "webhook not found"})
}

func SendWebhook(event string, data interface{}) {
	payload := WebhookEvent{
		Event:     event,
		Timestamp: time.Now(),
		Data:      data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal webhook payload: %v", err)
		return
	}

	webhooks.Range(func(key, value interface{}) bool {
		wh := value.(Webhook)
		for _, e := range wh.Events {
			if e == event {
				go sendWebhookRequest(wh.URL, body)
				break
			}
		}
		return true
	})
}

func sendWebhookRequest(url string, body []byte) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Failed to send webhook to %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Webhook to %s returned status %d", url, resp.StatusCode)
	}
}
