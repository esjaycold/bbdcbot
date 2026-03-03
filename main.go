package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// SlotRequest represents the JSON payload for scanning slots
type SlotRequest struct {
	CourseType      string      `json:"courseType"`
	InsInstructorId string      `json:"insInstructorId"`
	StageSubDesc    string      `json:"stageSubDesc"`
	SubStageSubNo   interface{} `json:"subStageSubNo"`
	SubVehicleType  interface{} `json:"subVehicleType"`
}

func main() {
	// Telegram setup
	botToken := os.Getenv("TELEGRAM_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_TOKEN not set")
	}
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Failed to start telegram bot: ", err)
	}

	chatIDStr := os.Getenv("CHAT_ID")
	if chatIDStr == "" {
		log.Fatal("CHAT_ID not set")
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("Invalid CHAT_ID: ", err)
	}

	log.Printf("Telegram bot authorized on account %s", bot.Self.UserName)

	// Heroku ping endpoint to keep alive
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Bot is running"))
		})
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}()

	// Get BBDC_TOKEN
	bbdcToken := os.Getenv("BBDC_TOKEN")
	if bbdcToken == "" {
		log.Fatal("BBDC_TOKEN not set")
	}

	// Load filters
	months := strings.Split(os.Getenv("WANTED_MONTHS"), ",")
	sessions := strings.Split(os.Getenv("WANTED_SESSIONS"), ",")
	days := strings.Split(os.Getenv("WANTED_DAYS"), ",")

	client := &http.Client{}

	for {
		log.Println("Scanning available slots...")

		// Construct payload for slot scan
		payload := SlotRequest{
			CourseType:      "3A",
			InsInstructorId: "",
			StageSubDesc:    "Practical Lesson",
			SubStageSubNo:   nil,
			SubVehicleType:  nil,
		}

		payloadBytes, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", os.Getenv("BBDC_LINK"), bytes.NewBuffer(payloadBytes))
		if err != nil {
			log.Println("Failed to create request:", err)
			time.Sleep(30 * time.Second)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+bbdcToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Println("Error fetching slots:", err)
			time.Sleep(30 * time.Second)
			continue
		}

		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		var data []map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			log.Println("Error parsing response:", err)
			time.Sleep(30 * time.Second)
			continue
		}

		foundSlot := false
		for _, slot := range data {
			// Example filtering: months, sessions, days
			slotMonth := slot["month"].(string)   // adapt based on response
			slotSession := fmt.Sprintf("%v", slot["session"])
			slotDay := fmt.Sprintf("%v", slot["day"])

			if contains(months, slotMonth) && contains(sessions, slotSession) && contains(days, slotDay) {
				msg := fmt.Sprintf("Available slot: Month=%s, Day=%s, Session=%s", slotMonth, slotDay, slotSession)
				alert(msg, bot, chatID)
				foundSlot = true
			}
		}

		if !foundSlot {
			log.Println("No matching slots found")
		}

		// Retrigger after random 2–5 minutes
		wait := time.Duration(120+randInt(0, 180)) * time.Second
		alert(fmt.Sprintf("Retrigger in: %v", wait), bot, chatID)
		time.Sleep(wait)
	}
}

// Telegram alert helper
func alert(msg string, bot *tgbotapi.BotAPI, chatID int64) {
	telegramMsg := tgbotapi.NewMessage(chatID, msg)
	_, err := bot.Send(telegramMsg)
	if err != nil {
		log.Println("Failed to send telegram message:", err)
	} else {
		log.Println("Sent message:", msg)
	}
}

// Helper for slice contains
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

// Random int helper
func randInt(min, max int) int {
	return min + int(time.Now().UnixNano()%int64(max-min+1))
}
